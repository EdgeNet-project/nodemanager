package user

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

// Exists checks if a user exists on the system.
func Exists(username string) (bool, error) {
	_, err := user.Lookup(username)
	if err == nil {
		return true, nil
	}
	if _, ok := err.(user.UnknownUserError); ok {
		return false, nil
	}
	return false, err
}

// Add adds a new user to the system without a password.
func Add(username string) error {
	// -m: create home directory
	// -s /bin/bash: set shell to bash
	// -U: create a group with the same name as the user
	return exec.Command("useradd", "-m", "-s", "/bin/bash", "-U", username).Run()
}

// AddToSudoersNoPasswd adds a user to sudoers with NOPASSWD.
func AddToSudoersNoPasswd(username string) error {
	sudoersLine := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", username)
	sudoersFile := fmt.Sprintf("/etc/sudoers.d/%s", username)

	return os.WriteFile(sudoersFile, []byte(sudoersLine+"\n"), 0440)
}

// AddSSHKey adds a public SSH key to the user's authorized_keys file.
func AddSSHKey(username string, publicKey string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}

	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}

	var iuid, igid int
	fmt.Sscanf(u.Uid, "%d", &iuid)
	fmt.Sscanf(u.Gid, "%d", &igid)

	if err := os.Chown(sshDir, iuid, igid); err != nil {
		return err
	}

	authorizedKeysFile := filepath.Join(sshDir, "authorized_keys")
	f, err := os.OpenFile(authorizedKeysFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(publicKey + "\n"); err != nil {
		return err
	}

	return os.Chown(authorizedKeysFile, iuid, igid)
}
