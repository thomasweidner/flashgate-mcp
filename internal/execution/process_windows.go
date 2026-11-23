package execution

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

func currentProcess(withGroups bool) (Identity, []GroupID, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return Identity{}, nil, err
	}
	defer func() { _ = token.Close() }()
	user, err := token.GetTokenUser()
	if err != nil {
		return Identity{}, nil, err
	}
	account, err := user.User.Sid.String()
	if err != nil {
		return Identity{}, nil, err
	}
	identity := Identity{Account: PrincipalID(account), Binding: strconv.Itoa(os.Getpid())}
	if !withGroups {
		return identity, nil, nil
	}
	groups, err := enabledTokenGroups(token)
	return identity, groups, err
}

func enabledTokenGroups(token syscall.Token) ([]GroupID, error) {
	var bytes uint32
	err := syscall.GetTokenInformation(token, syscall.TokenGroups, nil, 0, &bytes)
	const maxTokenGroupBytes = 1 << 20
	if err != syscall.ERROR_INSUFFICIENT_BUFFER || bytes < 4 || bytes > maxTokenGroupBytes {
		return nil, ErrMismatch
	}
	data := make([]byte, bytes)
	if err := syscall.GetTokenInformation(token, syscall.TokenGroups, &data[0], bytes, &bytes); err != nil {
		return nil, err
	}
	type tokenGroups struct {
		count uint32
		first syscall.SIDAndAttributes
	}
	offset := unsafe.Offsetof(tokenGroups{}.first)
	groupSize := unsafe.Sizeof(syscall.SIDAndAttributes{})
	head := (*tokenGroups)(unsafe.Pointer(&data[0]))
	if uintptr(bytes) < offset || uintptr(head.count) > (uintptr(bytes)-offset)/groupSize {
		return nil, ErrMismatch
	}
	const enabled = 0x00000004
	const denyOnly = 0x00000010
	var groups []GroupID
	for _, group := range unsafe.Slice(&head.first, head.count) {
		if group.Attributes&enabled == 0 || group.Attributes&denyOnly != 0 {
			continue
		}
		id, err := group.Sid.String()
		if err != nil {
			return nil, err
		}
		groups = append(groups, GroupID(id))
	}
	return groups, nil
}
