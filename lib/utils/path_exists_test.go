package utils

import "testing"

func TestCheckPathOnDirectory(t *testing.T) {
	if !Check("./") {
		t.Error("Check should return true for existing directory")
	}
}

func TestCheckPathOnFile(t *testing.T) {
	if !Check("./path_exists.go") {
		t.Error("Check should return true for existing file")
	}
}

func TestCheckPathOnFileNotExist(t *testing.T) {
	if Check("./path_exists_not_exist.go") {
		t.Error("Check should return false for non existing file")
	}
}

func TestCheckPathOnDirectoryNotExist(t *testing.T) {
	if Check("./dir_not_exist") {
		t.Error("Check should return true for non existing directory")
	}
}
