package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUidGidParser(t *testing.T) {

	_, _, err1 := stringToUidAndGid("")
	assert.NotNil(t, err1)

	_, _, err2 := stringToUidAndGid("a:b:c")
	assert.NotNil(t, err2)

	_, _, err3 := stringToUidAndGid("0:a")
	assert.NotNil(t, err3)

	_, _, err4 := stringToUidAndGid("a")
	assert.NotNil(t, err4)

	uid, gid, err5 := stringToUidAndGid("10")
	assert.Nil(t, err5)
	assert.Equal(t, 10, uid)
	assert.Equal(t, -1, gid)

	uid, gid, err6 := stringToUidAndGid("10:10")
	assert.Nil(t, err6)
	assert.Equal(t, 10, uid)
	assert.Equal(t, 10, gid)

}
