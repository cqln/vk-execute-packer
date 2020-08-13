package e2e

import (
	"os"
	"sync"
	"testing"

	"github.com/SevereCloud/vksdk/api"
	packer "github.com/b3q/vk-execute-packer"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
	token := os.Getenv("TOKEN")
	vk := api.NewVK(token)
	vk.Limit = api.LimitUserToken
	packer.Default(vk)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		resp, err := vk.UtilsResolveScreenName(api.Params{
			"screen_name": "durov",
		})
		assert.Nil(t, err)
		assert.Equal(t, 1, resp.ObjectID)
	}()
	go func() {
		defer wg.Done()
		_, err := vk.UtilsResolveScreenName(nil)
		assert.Errorf(t, err, "One of the parameters specified was missing or invalid: screen_name is undefined")
	}()
	go func() {
		defer wg.Done()
		_, err := vk.AccountGetInfo(nil)
		assert.Nil(t, err)
	}()
	wg.Wait()
}
