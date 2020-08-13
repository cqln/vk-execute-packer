package e2e

import (
	"os"
	"sync"
	"testing"

	"github.com/SevereCloud/vksdk/api"
	packer "github.com/b3q/vk-execute-packer"
	"github.com/stretchr/testify/assert"
)

func TestLargeAPICalls(t *testing.T) {
	token := os.Getenv("TOKEN")
	vk := api.NewVK(token)
	vk.Limit = api.LimitUserToken
	packer.Default(vk)
	var wg sync.WaitGroup
	num := 500
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func(i int) {
			defer wg.Done()
			resp, err := vk.UtilsResolveScreenName(api.Params{
				"screen_name": "durov",
			})
			assert.Nil(t, err)
			assert.Equal(t, 1, resp.ObjectID)
		}(i)
	}
	wg.Wait()
}
