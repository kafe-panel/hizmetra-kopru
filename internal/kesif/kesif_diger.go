//go:build !windows

package kesif

import "github.com/kafe-panel/hizmetra-kopru/internal/api"

// bulPlatform — Windows dışında yerel yazıcı keşfi yok (ajan Windows hedefli).
// Ağ yazıcıları panelden "ip:9100" olarak elle girilir, keşif gerektirmez.
func bulPlatform() ([]api.Yazici, error) {
	return []api.Yazici{}, nil
}
