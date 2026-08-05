//go:build windows

package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func SecurePrivateTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("unsafe file in Spare state: %s", path)
		}
		return secureWindowsPath(path, info.IsDir())
	})
}

func VerifyPrivateTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("unsafe file in Spare state: %s", path)
		}
		return verifyWindowsPath(path)
	})
}

func secureWindowsPath(path string, directory bool) error {
	userSID, err := currentUserSID()
	if err != nil {
		return err
	}
	inheritance := ""
	if directory {
		inheritance = "OICI"
	}
	descriptor, err := windows.SecurityDescriptorFromString(
		fmt.Sprintf("D:P(A;%s;FA;;;%s)(A;%s;FA;;;SY)", inheritance, userSID.String(), inheritance),
	)
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
}

func verifyWindowsPath(path string) error {
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return errors.New("the DACL inherits access from another directory")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if dacl == nil || dacl.AceCount != 2 {
		return errors.New("the private DACL does not contain exactly two entries")
	}
	userSID, err := currentUserSID()
	if err != nil {
		return err
	}
	systemSID, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		return err
	}
	foundUser := false
	foundSystem := false
	for index := uint32(0); index < uint32(dacl.AceCount); index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, index, &ace); err != nil {
			return err
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return errors.New("the private DACL contains a non-allow entry")
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		foundUser = foundUser || sid.Equals(userSID)
		foundSystem = foundSystem || sid.Equals(systemSID)
	}
	if !foundUser || !foundSystem {
		return errors.New("the private DACL is not restricted to the current user and SYSTEM")
	}
	return nil
}

func currentUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	if user == nil || user.User.Sid == nil {
		return nil, errors.New("the current Windows user SID is unavailable")
	}
	return user.User.Sid, nil
}
