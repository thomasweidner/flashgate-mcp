//go:build linux

package execution

import (
	"os"
	"strconv"
)

func currentProcess(withGroups bool) (Identity, []GroupID, error) {
	identity := Identity{Account: PrincipalID(strconv.Itoa(os.Geteuid())), Binding: strconv.Itoa(os.Getpid())}
	if !withGroups {
		return identity, nil, nil
	}
	groups, err := os.Getgroups()
	if err != nil {
		return Identity{}, nil, err
	}
	ids := make([]GroupID, 0, len(groups)+1)
	ids = append(ids, GroupID(strconv.Itoa(os.Getegid())))
	for _, group := range groups {
		ids = append(ids, GroupID(strconv.Itoa(group)))
	}
	return identity, ids, nil
}
