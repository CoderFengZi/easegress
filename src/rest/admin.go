package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"pipelines"
	"sort"

	"github.com/ant0ine/go-json-rest/rest"

	"common"
	"config"
	"engine"
	"logger"
	"model"
	"plugins"
)

type adminServer struct {
	gateway *engine.Gateway
}

func newAdminServer(gateway *engine.Gateway) (*adminServer, error) {
	return &adminServer{
		gateway: gateway,
	}, nil
}

func (s *adminServer) Api() (*rest.Api, error) {
	router, err := rest.MakeRouter(
		rest.Post(common.PrefixAPIVersion("/plugins"), s.createPlugin),
		rest.Get(common.PrefixAPIVersion("/plugins"), s.retrievePlugins),
		rest.Get(common.PrefixAPIVersion("/plugins/#pluginName"), s.retrievePlugin),
		rest.Put(common.PrefixAPIVersion("/plugins"), s.updatePlugin),
		rest.Delete(common.PrefixAPIVersion("/plugins/#pluginName"), s.deletePlugin),

		rest.Post(common.PrefixAPIVersion("/pipelines"), s.createPipeline),
		rest.Get(common.PrefixAPIVersion("/pipelines"), s.retrievePipelines),
		rest.Get(common.PrefixAPIVersion("/pipelines/#pipelineName"), s.retrievePipeline),
		rest.Put(common.PrefixAPIVersion("/pipelines"), s.updatePipeline),
		rest.Delete(common.PrefixAPIVersion("/pipelines/#pipelineName"), s.deletePipeline),

		rest.Get(common.PrefixAPIVersion("/plugin-types"), s.retrievePluginTypes),
		rest.Get(common.PrefixAPIVersion("/pipeline-types"), s.retrievePipelineTypes),
	)

	if err != nil {
		logger.Errorf("[make router for admin server failed: %s]", err)
		return nil, err
	}

	api := rest.NewApi()
	api.Use(rest.DefaultCommonStack...)
	api.SetApp(router)

	return api, nil
}

func (s *adminServer) createPlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[create plugin]")

	req := new(pluginCreationRequest)
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req == nil || req.Config == nil {
		msg := "invalid request"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	buff, err := json.Marshal(req.Config)
	if err != nil {
		msg := ("plugin config is invalid.")
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	conf, err := plugins.GetConfig(req.Type)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	err = json.Unmarshal(buff, conf)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	pluginName := conf.PluginName()

	plugin := s.gateway.Model().GetPlugin(pluginName)
	if plugin != nil {
		msg := fmt.Sprintf("plugin %s already exists.", pluginName)
		rest.Error(w, msg, http.StatusConflict)
		logger.Errorf("[%s]", msg)
		return
	}

	constructor, err := plugins.GetConstructor(req.Type)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	_, err = s.gateway.Model().AddPlugin(req.Type, conf, constructor)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugin %s created]", pluginName)
}

func (s *adminServer) retrievePlugins(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugins]")

	req := new(pluginsRetrieveRequest)
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plugs, err := s.gateway.Model().GetPlugins(req.NamePattern, req.Types)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	resp := new(pluginsRetrieveResponse)
	resp.Plugins = make([]config.PluginSpec, 0)

	for _, plugin := range plugs {
		spec := config.PluginSpec{
			Type:   plugin.Type(),
			Config: plugin.Config(),
		}
		resp.Plugins = append(resp.Plugins, spec)
	}

	w.WriteJson(resp)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugins returned]")
}

func (s *adminServer) retrievePlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve plugin]")

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plugin := s.gateway.Model().GetPlugin(pluginName)
	if plugin == nil {
		msg := fmt.Sprintf("plugin %s not found", pluginName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	resp := config.PluginSpec{
		Type:   plugin.Type(),
		Config: plugin.Config(),
	}

	w.WriteJson(resp)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugin %s returned]", pluginName)
}

func (s *adminServer) updatePlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[update plugin]")

	req := new(pluginUpdateRequest)
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	buff, err := json.Marshal(req.Config)
	if err != nil {
		msg := ("plugin config is invalid.")
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	conf, err := plugins.GetConfig(req.Type)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	err = json.Unmarshal(buff, conf)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	pluginName := conf.PluginName()

	plugin := s.gateway.Model().GetPlugin(pluginName)
	if plugin == nil {
		msg := fmt.Sprintf("plugin %s not found", pluginName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	if plugin.Type() != req.Type {
		msg := fmt.Sprint("plugin type is readonly.")
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	err = s.gateway.Model().UpdatePluginConfig(conf)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[the config of plugin %s updated]", pluginName)
}

func (s *adminServer) deletePlugin(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[delete plugin]")

	pluginName, err := url.QueryUnescape(r.PathParam("pluginName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plugin := s.gateway.Model().GetPlugin(pluginName)
	if plugin == nil {
		msg := fmt.Sprintf("plugin %s not found", pluginName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	err = s.gateway.Model().DismissPluginInstance(pluginName)
	if err != nil {
		msg := err.Error()
		rest.Error(w, msg, http.StatusInternalServerError) // Doesn't make sense
		logger.Errorf("[%s]", msg)
		return
	}

	err = s.gateway.Model().DeletePlugin(pluginName)
	if err != nil {
		msg := err.Error()
		rest.Error(w, msg, http.StatusNotAcceptable)
		logger.Errorf("[%s]", msg)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugin %s deleted]", pluginName)
}

func (s *adminServer) createPipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[create pipeline]")

	req := new(pipelineCreationRequest)
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	if req == nil || req.Config == nil {
		msg := "invalid request"
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	buff, err := json.Marshal(req.Config)
	if err != nil {
		msg := ("pipeline config is invalid.")
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	conf, err := model.GetPipelineConfig(req.Type)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	err = json.Unmarshal(buff, conf)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	pipelineName := conf.PipelineName()

	pipeline := s.gateway.Model().GetPipeline(pipelineName)
	if pipeline != nil {
		msg := fmt.Sprintf("pipeline %s already exists", pipelineName)
		rest.Error(w, msg, http.StatusConflict)
		logger.Errorf("[%s]", msg)
		return
	}

	_, err = s.gateway.Model().AddPipeline(req.Type, conf)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[pipeline %s created]", pipelineName)
}

func (s *adminServer) retrievePipelines(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipelines]")

	req := new(pipelinesRetrieveRequest)
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pipes, err := s.gateway.Model().GetPipelines(req.NamePattern, req.Types)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	resp := new(pipelinesRetrieveResponse)
	resp.Pipelines = make([]config.PipelineSpec, 0)

	for _, pipeline := range pipes {
		spec := config.PipelineSpec{
			Type:   pipeline.Type(),
			Config: pipeline.Config(),
		}
		resp.Pipelines = append(resp.Pipelines, spec)
	}

	w.WriteJson(resp)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[pipelines returned]")
}

func (s *adminServer) retrievePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[retrieve pipeline]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pipeline := s.gateway.Model().GetPipeline(pipelineName)
	if pipeline == nil {
		msg := fmt.Sprintf("pipeline %s not found", pipelineName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	resp := config.PipelineSpec{
		Type:   pipeline.Type(),
		Config: pipeline.Config(),
	}

	w.WriteJson(resp)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[pipeline %s returned]", pipelineName)
}

func (s *adminServer) updatePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[update pipeline]")

	req := new(pipelineUpdateRequest)
	err := r.DecodeJsonPayload(req)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	buff, err := json.Marshal(req.Config)
	if err != nil {
		msg := fmt.Sprintf("pipeline config %s is invalid.", req.Type)
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	conf, err := model.GetPipelineConfig(req.Type)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	err = json.Unmarshal(buff, conf)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	pipelineName := conf.PipelineName()

	pipeline := s.gateway.Model().GetPipeline(pipelineName)
	if pipeline == nil {
		msg := fmt.Sprintf("pipeline %s not found", pipelineName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	if pipeline.Type() != req.Type {
		msg := fmt.Sprint("pipeline type is readonly.")
		rest.Error(w, msg, http.StatusBadRequest)
		logger.Errorf("[%s]", msg)
		return
	}

	err = s.gateway.Model().UpdatePipelineConfig(conf)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[the config of pipeline %s updated]", pipelineName)
}

func (s *adminServer) deletePipeline(w rest.ResponseWriter, r *rest.Request) {
	logger.Debugf("[delete pipeline]")

	pipelineName, err := url.QueryUnescape(r.PathParam("pipelineName"))
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pipeline := s.gateway.Model().GetPipeline(pipelineName)
	if pipeline == nil {
		msg := fmt.Sprintf("pipeline %s not found", pipelineName)
		rest.Error(w, msg, http.StatusNotFound)
		logger.Debugf("[%s]", msg)
		return
	}

	err = s.gateway.Model().DeletePipeline(pipelineName)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusBadRequest)
		logger.Errorf("[%v]", err)
		return
	}

	w.WriteHeader(http.StatusOK)

	logger.Debugf("[pipeline %s deleted]", pipelineName)
}

func (s *adminServer) retrievePluginTypes(w rest.ResponseWriter, _ *rest.Request) {
	logger.Debugf("[retrieve plugin types]")

	resp := new(pluginTypesRetrieveResponse)
	resp.PluginTypes = make([]string, 0)

	for _, typ := range plugins.GetAllTypes() {
		// Defensively
		if !common.StrInSlice(typ, resp.PluginTypes) {
			resp.PluginTypes = append(resp.PluginTypes, typ)
		}
	}

	// Returns with stable order
	sort.Strings(resp.PluginTypes)

	w.WriteJson(resp)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[plugin types returned]")
}

func (s *adminServer) retrievePipelineTypes(w rest.ResponseWriter, _ *rest.Request) {
	logger.Debugf("[retrieve pipeline types]")

	resp := new(pipelineTypesRetrieveResponse)
	resp.PipelineTypes = append(resp.PipelineTypes, pipelines.GetAllTypes()...)

	// Returns with stable order
	sort.Strings(resp.PipelineTypes)

	w.WriteJson(resp)
	w.WriteHeader(http.StatusOK)

	logger.Debugf("[pipeline types returned]")
}