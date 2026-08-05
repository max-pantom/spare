//go:build !windows

package paths

func SecurePrivateTree(string) error {
	return nil
}

func VerifyPrivateTree(string) error {
	return nil
}
