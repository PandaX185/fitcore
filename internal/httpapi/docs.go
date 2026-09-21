package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/swaggest/swgui/v5emb"

	"github.com/PandaX185/fitcore/api"
)

// mountDocs serves the OpenAPI spec and an interactive Swagger UI.
func mountDocs(r *gin.Engine) {
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", api.OpenAPIYAML)
	})

	ui := v5emb.New("FitCore API", "/openapi.yaml", "/swagger/")
	r.Any("/swagger/*any", gin.WrapH(ui))
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/swagger/")
	})
}
