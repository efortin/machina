package utils

import "os"

func DirectoryCreateIfAbsent(path string) (err error) {
	_, err = os.Stat(path)
	if err != nil {
		err = os.Mkdir(path, os.ModePerm)
		Logger.Debug(path, "not exist, has been created")
	}
	return err

}
