//go:build !unix

package wrapper

func groupHasListeningPort(int) bool { return false }
