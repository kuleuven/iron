package iron

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/kuleuven/iron/scramble"
)

var ErrAuthFilePathNotSet = errors.New("auth file path is not set")

// ReadAuthFile reads the contents of a file and decodes it according to the
// iRODS scramble algorithm. If no uid is given, the uid of the file is used
// to decode the contents.
// The file is expected to have been created with the iRODS `iinit` command.
func ReadAuthFile(authFile string, uid *int) (string, error) {
	if authFile == "" {
		return "", ErrAuthFilePathNotSet
	}

	f, err := os.Open(authFile)
	if err != nil {
		return "", err
	}

	defer f.Close()

	var descrambleUID int

	if uid == nil {
		fi, err := f.Stat()
		if err != nil {
			return "", err
		}

		descrambleUID = uidOfFile(fi)
	} else {
		descrambleUID = *uid
	}

	encoded, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return scramble.DecodeIrodsA(encoded, descrambleUID)
}

// WriteAuthFile writes the given password to a file using the iRODS
// scramble algorithm. The file is expected to be the .irodsA file
// used by the iinit command. The file is created if it doesn't exist,
// and the permissions are set to 0600. If the file is already present,
// the content is overwritten. The uid of the file owner is used to
// encode the password.
func WriteAuthFile(authFile string, uid *int, password string) error {
	if authFile == "" {
		return ErrAuthFilePathNotSet
	}

	f, err := os.OpenFile(authFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	defer f.Close()

	var scrambleUID int

	if uid == nil {
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		scrambleUID = uidOfFile(fi)
	} else {
		scrambleUID = *uid
	}

	_, err = f.Write(scramble.EncodeIrodsA(password, scrambleUID, time.Now()))

	return err
}
