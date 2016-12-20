package generateid

import (
	"github.com/valyala/fasthttp"
	"gopeerjs-server/config"
	"regexp"
	"gopeerjs-server/peerhub"
	"fmt"
)

var (
	rePath, _ = regexp.Compile("^" + config.AppBindPath + "/([^/]+)/id$")
	logger = config.Logger
	debug = config.Debug
	handlerName = "generateid"
)

func Match(ctx *fasthttp.RequestCtx) bool {
	if ctx.IsGet() {
		if m := rePath.FindSubmatch(ctx.URI().Path()); len(m) > 0 {
			ctx.SetUserValue("key", m[1])

			if debug {
				logger.Infof("[%s] Match. %s %s", handlerName, ctx.Method(), ctx.URI())
			}

			return true
		}
	}
	return false
}

func Handle(ctx *fasthttp.RequestCtx) {
	keyToken, _ := ctx.UserValue("key").(string)

	if _, err := peerhub.Token2Key(keyToken); err != nil {
		if debug {
			logger.Errorf("[%s] %s", handlerName, err)
		}

		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}


	uuid := peerhub.GeneratePeerId()

	ctx.SetContentType("text/html")
	ctx.SetBodyString(fmt.Sprintf("%s", uuid))
}