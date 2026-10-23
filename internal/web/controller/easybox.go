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
	e.GET("/link/:id", a.endpointLink)
	e.POST("/setInSubscription/:id", a.setEndpointInSubscription)
	e.POST("/regenerate/:id", a.regenerateEndpoint)
	e.POST("/rename/:id", a.renameEndpoint)
	g.GET("/templates", a.listTemplates)
	g.GET("/subscription/preview", a.previewSubscription)
	g.POST("/subscription/rotate", a.rotateSubscription)
	g.GET("/subscription/info", a.subscriptionInfo)
	g.POST("/subscription/cdn", a.setSubscriptionCDN)
	g.POST("/subscription/tierLabels", a.setSubscriptionTierLabels)
	g.GET("/cloudflare", a.getCloudflare)
	g.POST("/cloudflare", a.setCloudflare)
	g.POST("/cloudflare/clear", a.clearCloudflare)
	g.GET("/region", a.getRegion)
	g.POST("/region", a.setRegion)
	g.GET("/preferred", a.getPreferred)
	g.POST("/preferred", a.setPreferred)
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

func (a *EasyboxController) setEndpointInSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var f struct {
		Include bool `json:"include" form:"include"`
	}
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "", a.svc.SetEndpointInSubscription(id, f.Include))
}

// subscriptionInfo builds URLs for the host the admin reached the panel on,
// matching how the panel's own settings fill in subURI.
func (a *EasyboxController) subscriptionInfo(c *gin.Context) {
	view, err := a.svc.GetSubscription(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) endpointLink(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	l, err := a.svc.GetEndpointLink(id, c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, l, nil)
}

func (a *EasyboxController) regenerateEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var f struct {
		KeepOld bool `json:"keepOld" form:"keepOld"`
	}
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	ep, err := a.svc.RegenerateEndpoint(id, f.KeepOld)
	if err != nil {
		jsonMsgObj(c, I18nWeb(c, "somethingWentWrong"), ep, err)
		return
	}
	jsonObj(c, ep, nil)
}

func (a *EasyboxController) rotateSubscription(c *gin.Context) {
	if _, err := a.svc.RotateSubscription(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	view, err := a.svc.GetSubscription(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) listTemplates(c *gin.Context) {
	jsonObj(c, a.svc.ListTemplates(), nil)
}

func (a *EasyboxController) previewSubscription(c *gin.Context) {
	view, err := a.svc.PreviewSubscription(c.Query("client"), c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) getCloudflare(c *gin.Context) {
	view, err := a.svc.GetCloudflare()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) setCloudflare(c *gin.Context) {
	var req easybox.CloudflareRequest
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	view, err := a.svc.SetCloudflare(req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) clearCloudflare(c *gin.Context) {
	jsonMsg(c, "", a.svc.ClearCloudflare())
}

func (a *EasyboxController) getPreferred(c *gin.Context) {
	view, err := a.svc.GetPreferred()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) setPreferred(c *gin.Context) {
	var req easybox.CloudflarePreferredRequest
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	view, err := a.svc.SetPreferred(req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

// setSubscriptionCDN turns the Cloudflare subscription address on or off and
// answers with the subscription view, like rotate.
func (a *EasyboxController) setSubscriptionCDN(c *gin.Context) {
	var f struct {
		Enable bool `json:"enable" form:"enable"`
	}
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.svc.SetSubscriptionCDN(f.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	view, err := a.svc.GetSubscription(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) setSubscriptionTierLabels(c *gin.Context) {
	var f struct {
		Enable bool `json:"enable" form:"enable"`
	}
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.svc.SetTierLabels(f.Enable); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	view, err := a.svc.GetSubscription(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}

func (a *EasyboxController) renameEndpoint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var f struct {
		Name string `json:"name" form:"name"`
	}
	if err := c.ShouldBind(&f); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, "", a.svc.RenameEndpoint(id, f.Name))
}

func (a *EasyboxController) getRegion(c *gin.Context) {
	jsonObj(c, a.svc.GetRegion(), nil)
}

func (a *EasyboxController) setRegion(c *gin.Context) {
	var req easybox.RegionRequest
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	view, err := a.svc.SetRegion(req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, view, nil)
}
