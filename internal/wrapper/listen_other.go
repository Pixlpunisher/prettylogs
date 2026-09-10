//go:build !unix

package wrapper

func groupHasListeningPort(int) bool { return false }

func pidHasListeningPort(int) bool { return false }
