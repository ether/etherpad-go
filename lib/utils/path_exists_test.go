package utils

import "testing"

func TestCheckPathOnDirectory(t *testing.T) {
	if !ExistsSync("./") {
		t.Error("ExistsSync should return true for existing directory")
	}
}

func TestCheckPathOnFile(t *testing.T) {
	if !ExistsSync("./path_exists.go") {
		t.Error("ExistsSync should return true for existing file")
	}
}

func TestCheckPathOnFileNotExist(t *testing.T) {
	if ExistsSync("./path_exists_not_exist.go") {
		t.Error("ExistsSync should return false for non existing file")
	}
}

func TestCheckPathOnDirectoryNotExist(t *testing.T) {
	if ExistsSync("./dir_not_exist") {
		t.Error("ExistsSync should return true for non existing directory")
	}
}
