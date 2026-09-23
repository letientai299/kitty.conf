package kitty

import "crypto/md5"

var colors = [...]uint32{0x50fa7b, 0xff79c6, 0x8be9fd, 0xffb86c, 0xbd93f9, 0xf1fa8c, 0xff6e6e, 0x69ff94, 0xa4ffff, 0xff92df, 0xffd580, 0xcaa9fa}

func Color(name string) uint32 {
	hash := md5.Sum([]byte(name))
	index := 0
	for _, b := range hash {
		index = (index*256 + int(b)) % len(colors)
	}
	return colors[index]
}
