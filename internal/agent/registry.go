package agent

import (
	"fmt"
	"sort"
)

type Registry struct {
	agents map[string]Definition
}

func EmptyRegistry() *Registry {
	r := &Registry{agents: map[string]Definition{}}
	r.Reset()
	return r
}

func (r *Registry) Register(agent Definition) {
	r.agents[agent.AgentType] = agent
}

func (r *Registry) Reset() {
	r.agents = map[string]Definition{}
	for _, agent := range Builtins() {
		r.Register(agent)
	}
}

func (r *Registry) Get(agentType string) (Definition, error) {
	agent, ok := r.agents[agentType]
	if !ok {
		return Definition{}, fmt.Errorf("unknown agent type: %s", agentType)
	}
	return agent, nil
}

func (r *Registry) List() []Definition {
	out := make([]Definition, 0, len(r.agents))
	for _, agent := range r.agents {
		out = append(out, agent)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AgentType < out[j].AgentType
	})
	return out
}
