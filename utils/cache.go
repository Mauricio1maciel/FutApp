package utils

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var AppCache = cache.New(30*time.Second, 5*time.Minute)
