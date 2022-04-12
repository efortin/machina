package internal

import (
	"compress/gzip"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/cavaliergopher/grab/v3"
	"github.com/efortin/machina/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

const (
	Default_cpu_number = 2
	Default_mem_mb     = 2 * GB
	GB                 = 1024 * 1024 * 1024
)

type UbuntuDistribution struct {
	ReleaseName  string `json:"release"`
	Architecture string `json:"arch"`
}

func FromEnvWithDefault(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func GenerateAlmostUniqueMac(name string) string {
	_md5string := GetMD5Hash(name)
	return strings.Join([]string{"02", _md5string[0:2], _md5string[2:4], _md5string[4:6], _md5string[6:8], _md5string[8:10]}, ":")
}

func GetVirtualMachineDirectory(vmName string) string {
	return fmt.Sprintf("%s/%s", GetWorkingDirectory(), vmName)
}

func readPidFromFile(filename string) (int, error) {
	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	content := strings.TrimSpace(string(bs))
	pid, err := strconv.Atoi(content)
	if err != nil {
		return 0, fmt.Errorf("%v parsing %s", filename)
	}

	return pid, nil
}

func (release *UbuntuDistribution) ImageDirectory() string {
	return fmt.Sprintf("%s/%s", baseImageDirectory(), release.ReleaseName)
}

func (release *UbuntuDistribution) cloneIfNotExist(srcFilePath string, dstFilePath string) (err error) {
	if _, err := os.Stat(dstFilePath); err == nil {
		utils.Logger.Infof("Machine files %s exists, ignore copy", dstFilePath)
		return err
	}
	err = unix.Clonefile(srcFilePath, dstFilePath, 0)
	return
}

func baseMachineDirectory() string {
	baseMachineDirectory := fmt.Sprintf("%s/machines", GetWorkingDirectory())
	DirectoryCreateIfAbsent(baseMachineDirectory)
	return baseMachineDirectory
}

func baseImageDirectory() string {
	imageDirectory := fmt.Sprintf("%s/images", GetWorkingDirectory())
	DirectoryCreateIfAbsent(imageDirectory)
	return imageDirectory
}

func ListExistingMachines() *utils.Set {
	files, err := os.ReadDir(baseMachineDirectory())
	directoryNameStrings := make([]string, 0)
	if err != nil {
		return utils.NewSet()
	}
	for _, file := range files {
		directoryNameStrings = append(directoryNameStrings, file.Name())
	}
	return utils.NewSetFromArray(directoryNameStrings)
}

func FromFileSpec(name string) (*Machine, error) {

	specsFile, err := os.Open(InfoFilePath(name))
	if err != nil {
		return nil, fmt.Errorf("the machine %s doesn't not exist. will be created", name)
	}

	specsByteArray, _ := ioutil.ReadAll(specsFile)
	var machine Machine
	err = json.Unmarshal(specsByteArray, &machine)

	return &machine, err
}

// DownloadDistro will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func (r *UbuntuDistribution) DownloadDistro() (err error) {
	DirectoryCreateIfAbsent(r.ImageDirectory())
	err = r.downloadInitRd()
	err = r.downloadKernel()
	err = r.downloadImage()
	return err

}

func (release *UbuntuDistribution) InitRdPath() string {
	return release.ImageDirectory() + fmt.Sprintf("/%s-server-cloudimg-%s-initrd-generic", release.ReleaseName, release.Architecture)
}

func (release *UbuntuDistribution) KernelPath() string {
	return release.ImageDirectory() + fmt.Sprintf("/%s-server-cloudimg-%s-vmlinuz-generic", release.ReleaseName, release.Architecture)
}

func (release *UbuntuDistribution) KernelPathGZIP() string {
	return release.ImageDirectory() + fmt.Sprintf("/%s-server-cloudimg-%s-vmlinuz-generic.gz", release.ReleaseName, release.Architecture)
}

func (release *UbuntuDistribution) ImagePath() string {
	return release.ImageDirectory() + fmt.Sprintf("/%s-server-cloudimg-%s.img", release.ReleaseName, release.Architecture)
}

func (release *UbuntuDistribution) downloadInitRd() (err error) {
	_, err = os.Stat(release.InitRdPath())
	if err == nil {
		utils.Logger.Infof("InitRD %s at %s already exists", release.ReleaseName, release.InitRdPath())
		return
	}
	_, err = grab.Get(release.ImageDirectory(), fmt.Sprintf("https://cloud-images.ubuntu.com/%s/current/unpacked/%s-server-cloudimg-%s-initrd-generic", release.ReleaseName, release.ReleaseName, release.Architecture))
	return err
}

func (release *UbuntuDistribution) downloadKernel() (err error) {
	_, err = os.Stat(release.KernelPath())
	if err == nil {
		utils.Logger.Infof("Kernel %s at %s already exists", release.ReleaseName, release.KernelPath())
		return
	}
	_, err = grab.Get(release.KernelPathGZIP(), fmt.Sprintf("https://cloud-images.ubuntu.com/%s/current/unpacked/%s-server-cloudimg-%s-vmlinuz-generic", release.ReleaseName, release.ReleaseName, release.Architecture))

	if err != nil {
		fmt.Println("ERRRRR", err)
	}
	// Open compressed file
	gzipFile, err := os.Open(release.KernelPathGZIP())
	if err != nil {
		log.Fatal(err)
	}

	// Create a gzip reader on top of the file reader
	// Again, it could be any type reader though
	gzipReader, err := gzip.NewReader(gzipFile)
	if err != nil {
		log.Fatal(err)
	}
	defer gzipReader.Close()

	// Uncompress to a writer. We'll use a file writer
	outfileWriter, err := os.Create(release.KernelPath())
	if err != nil {
		log.Fatal(err)
	}
	defer outfileWriter.Close()

	// Copy contents of gzipped file to output file
	_, err = io.Copy(outfileWriter, gzipReader)
	if err != nil {
		log.Fatal(err)
	}

	return err
}

func (release *UbuntuDistribution) downloadImage() (err error) {

	_, err = os.Stat(release.ImagePath())
	if err == nil {
		utils.Logger.Debugf("Image %s at %s already exists", release.ReleaseName, release.ImagePath())
		return
	}
	_, err = grab.Get(release.ImageDirectory(), fmt.Sprintf("https://cloud-images.ubuntu.com/%s/current/%s-server-cloudimg-%s.tar.gz", release.ReleaseName, release.ReleaseName, release.Architecture))

	fmt.Println(err)
	cmd := exec.Command("/usr/bin/tar", "xf", release.ImageDirectory()+"/"+release.ReleaseName+"-server-cloudimg-arm64.tar.gz")
	cmd.Dir = release.ImageDirectory() + "/"
	utils.Logger.Info("cmd directory", cmd.Dir)
	err = cmd.Run()
	cmd.Wait()

	return
}

func DirectoryCreateIfAbsent(path string) (err error) {
	_, err = os.Stat(path)
	if err != nil {
		err = os.Mkdir(path, os.ModePerm)
		utils.Logger.Debug(path, "not exist, has been created")
	}
	return err
}

func GetWorkingDirectory() string {
	user, err := user.Current()
	if err != nil {
		fmt.Errorf("%s", err)
		os.Exit(1)
	}
	vmctldir := FromEnvWithDefault("VMCTLDIR", fmt.Sprint(user.HomeDir, "/.vm"))
	DirectoryCreateIfAbsent(vmctldir)
	return vmctldir
}

func TmpDirectory() string {
	tmpdir := os.Getenv("TMPDIR")
	if tmpdir == utils.Empty {
		tmpDir := fmt.Sprintf("%s/%s", GetWorkingDirectory(), "/.tmp")
		err := DirectoryCreateIfAbsent(tmpDir)
		if err != nil {
			utils.Logger.Errorf("Cannot create the fallback temporary dir %s", tmpDir)
		}
		return tmpDir
	}
	return tmpdir
}

func GetMachinaPublicKey() (string, error) {
	bytes, err := ioutil.ReadFile(getMachinaPublicKeyPath())
	if err != nil {
		utils.Logger.Errorf("Error reading public key: %s", getMachinaPublicKeyPath())
		return utils.Empty, err
	}
	return string(bytes), nil
}

func getMachinaPrivateKeyPath() string {
	return fmt.Sprintf("%s/machina", GetWorkingDirectory())
}

func getMachinaPublicKeyPath() string {
	return fmt.Sprintf("%s/machina.pub", GetWorkingDirectory())
}

func GenerateMachinaKeypair() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.OpenFile(getMachinaPrivateKeyPath(), os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	defer privateKeyFile.Close()
	if err != nil {
		return err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(getMachinaPublicKeyPath(), ssh.MarshalAuthorizedKey(pub), 0655)
}
