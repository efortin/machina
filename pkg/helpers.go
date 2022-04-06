package internal

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/cavaliergopher/grab/v3"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"
)

type UbuntuDistribution struct {
	ReleaseName  string
	Architecture string
}

func Getenv(key, fallback string) string {
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
	return fmt.Sprintf("%s/%s", getWorkingDirectory(), vmName)
}

func getWorkingDirectory() string {
	user, err := user.Current()
	if err != nil {
		fmt.Errorf("%s", err)
		os.Exit(1)
	}

	vmctldir := Getenv("VMCTLDIR", user.HomeDir+"/.vm")
	if _, err := os.Stat(vmctldir); errors.Is(err, os.ErrNotExist) {
		klog.Warning("The .vm folder", vmctldir, " was created")
		if err := os.Mkdir(user.HomeDir+"/.vm", os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}
	return vmctldir
}

func (release *UbuntuDistribution) ImageDirectory() string {
	return fmt.Sprintf("%s/%s", release.baseImageDirectory(), release.ReleaseName)
}

func (release *UbuntuDistribution) copyFileIfNotExist(srcFilePath string, dstFilePath string) (err error) {
	if _, err := os.Stat(dstFilePath); err == nil {
		klog.Infof("Machine files %s exists, ignore copy", dstFilePath)
		return err
	}

	input, err := ioutil.ReadFile(srcFilePath)
	if err != nil {
		klog.Error(err)
		return
	}

	err = ioutil.WriteFile(dstFilePath, input, 0777)
	if err != nil {
		klog.Error("Error creating", dstFilePath)
	}
	time.Sleep(2 * time.Second)
	return
}

func (release *UbuntuDistribution) baseMachineDirectory() string {
	baseMachineDirectory := fmt.Sprintf("%s/machines", getWorkingDirectory())

	if _, err := os.Stat(baseMachineDirectory); errors.Is(err, os.ErrNotExist) {
		log.Default().Println("The root machine directory was not found, creating it...")
		if err := os.Mkdir(baseMachineDirectory, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}
	return baseMachineDirectory
}

func (release *UbuntuDistribution) baseImageDirectory() string {
	imageDirectory := fmt.Sprintf("%s/images", getWorkingDirectory())

	if _, err := os.Stat(imageDirectory); errors.Is(err, os.ErrNotExist) {
		log.Default().Println("Image directory not found, creating it...")
		if err := os.Mkdir(imageDirectory, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}
	return imageDirectory
}

// DownloadDistro will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func (r *UbuntuDistribution) DownloadDistro() (err error) {
	if err := os.Mkdir(r.ImageDirectory(), os.ModePerm); err != nil {
		klog.Error(err)
	}

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
		klog.Infof("InitRD %s at %s already exists", release.ReleaseName, release.InitRdPath())
		return
	}
	_, err = grab.Get(release.ImageDirectory(), fmt.Sprintf("https://cloud-images.ubuntu.com/%s/current/unpacked/%s-server-cloudimg-%s-initrd-generic", release.ReleaseName, release.ReleaseName, release.Architecture))
	return err
}

func (release *UbuntuDistribution) downloadKernel() (err error) {
	_, err = os.Stat(release.KernelPath())
	if err == nil {
		klog.Infof("Kernel %s at %s already exists", release.ReleaseName, release.KernelPath())
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
		klog.Infof("Image %s at %s already exists", release.ReleaseName, release.ImagePath())
		return
	}
	_, err = grab.Get(release.ImageDirectory(), fmt.Sprintf("https://cloud-images.ubuntu.com/%s/current/%s-server-cloudimg-%s.tar.gz", release.ReleaseName, release.ReleaseName, release.Architecture))

	cmd := exec.Command("/usr/bin/tar", "xf", release.ImageDirectory()+"/focal-server-cloudimg-arm64.tar.gz")
	cmd.Dir = release.ImageDirectory() + "/"
	klog.Infoln("cmd directory", cmd.Dir)
	err = cmd.Run()
	cmd.Wait()

	return
}
