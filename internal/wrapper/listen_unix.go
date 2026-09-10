//go:build unix

package wrapper

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func groupHasListeningPort(pgid int) bool {
	if lsofListening("-g", strconv.Itoa(pgid)) {
		return true
	}
	return procGroupListening(pgid)
}

func pidHasListeningPort(pid int) bool {
	if lsofListening("-p", strconv.Itoa(pid)) {
		return true
	}
	return procListening(pid)
}

func lsofListening(flag, value string) bool {
	cmd := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", flag, value)
	out, err := cmd.Output()
	return err == nil && len(bytes.TrimSpace(out)) > 0
}

func procGroupListening(pgid int) bool {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(ent.Name())
		if err != nil {
			continue
		}
		got, ok := procPgid(pid)
		if !ok || got != pgid {
			continue
		}
		if procListening(pid) {
			return true
		}
	}
	return false
}

func procPgid(pid int) (int, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, false
	}
	// comm can contain spaces and is wrapped in parentheses; pgid is field 5
	// after the closing paren.
	s := string(data)
	i := strings.LastIndex(s, ")")
	if i < 0 || i+1 >= len(s) {
		return 0, false
	}
	fields := strings.Fields(s[i+1:])
	if len(fields) < 3 {
		return 0, false
	}
	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, false
	}
	return pgid, true
}

func procListening(pid int) bool {
	inodes := socketInodes(pid)
	if len(inodes) == 0 {
		return false
	}
	return tcpListenHasInode("/proc/net/tcp", inodes) || tcpListenHasInode("/proc/net/tcp6", inodes)
}

func socketInodes(pid int) map[string]struct{} {
	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return nil
	}
	inodes := make(map[string]struct{})
	for _, fd := range fds {
		target, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
		if err != nil {
			continue
		}
		const prefix = "socket:["
		if !strings.HasPrefix(target, prefix) || !strings.HasSuffix(target, "]") {
			continue
		}
		inodes[target[len(prefix):len(target)-1]] = struct{}{}
	}
	return inodes
}

func tcpListenHasInode(path string, inodes map[string]struct{}) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return false
	}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// sl local rem st ... inode (inode is field 9)
		if len(fields) < 10 {
			continue
		}
		if fields[3] != "0A" {
			continue
		}
		if _, ok := inodes[fields[9]]; ok {
			return true
		}
	}
	return false
}
