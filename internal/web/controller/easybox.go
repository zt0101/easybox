package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/easybox"
)

// EasyboxController serves /panel/api/easybox: the EasyBox product layer
// (EasyBox.md §68). Messages reuse existing i18n keys; the UI keys off failedStep.
type EasyboxController struct {
	svc *easybox.Service
}

func NewEasyboxController(g *gin.RouterGroup) *EasyboxController {
	a := &EasyboxController{svc: easybox.NewService()}
	a.initRouter(g)
	return a
}

func (a *EasyboxController) initRouter(g *gin.RouterGroup) {
	e := g.Group("/endpoints")
	e.GET("/list", a.listEndpoints)
	e.POST("/create", a.createEndpoint)
	e.POST("/del/:id", a.delEndpoint)
	e.POST("/setEnable/:id", a.setEndpointEnable)
}

func (a *EasyboxController) listEndpoints(c *gin.Context) {
	list, err := a.svc.ListEndpoints()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, list, nil)
}

func (a *EasyboxController) createEndpoint(c *gin.Context) {
	var req easybox.CreateEndpointRequest
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	ep, err := a.svc.CreateEndpoint(req)
	if err != nil {
		jsonMsgObj(c, I18nWeb(c, "somethingWentWrong"), ep, err)
		return
	}
	jsonObj(c, ep, nil)
}

func (a *EasyboxController) delEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "", a.svc.DeleteEndpoint(id))
}

func (a *EasyboxController) setEndpointEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var f struct {
		Enable bool `json:"enable" form:"enable"`
	}
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "", a.svc.SetEndpointEnable(id, f.Enable))
}
