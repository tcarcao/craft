package view

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
)

// C4Options selects what a C4 container diagram shows.
type C4Options struct {
	// Boundaries draws each bounded context as its own container inside its
	// service; otherwise a service is one application container.
	Boundaries bool
	// ShowDatabases draws each service's data stores.
	ShowDatabases bool
	// FocusServices and FocusContexts, when either is set, keep the named
	// services (and the services deploying the named contexts) internal and
	// draw every other service as external.
	FocusServices, FocusContexts []string
}

// C4 is a C4 container diagram.
type C4 struct {
	Title     string
	Actors    []C4Actor
	Systems   []C4System // external systems first, then internal, each by name
	Relations []C4Relation
	// Languages lists the language icons the diagram uses, sorted.
	Languages []string
}

// C4ActorElement is how an actor is drawn.
type C4ActorElement string

const (
	C4Person         C4ActorElement = "person"
	C4ExternalSystem C4ActorElement = "external_system"
)

// C4Actor is a person or outside system that triggers a use case.
type C4Actor struct {
	ID, Label, Description string
	Element                C4ActorElement
}

// C4System is a service or an infrastructure layer.
type C4System struct {
	ID, Label string
	External  bool
	// Groups is set for a service in boundaries mode: one group per bounded
	// context. Containers holds everything else (or everything, otherwise).
	Groups     []C4Group
	Containers []C4Container
}

// C4Group is one bounded context inside a service, in boundaries mode.
type C4Group struct {
	ID, Label  string
	Containers []C4Container
}

// C4ContainerKind tells a writer which element to use.
type C4ContainerKind string

const (
	C4App      C4ContainerKind = "app"
	C4Database C4ContainerKind = "database"
	C4Queue    C4ContainerKind = "queue"
)

// C4Container is one deployable piece.
type C4Container struct {
	ID, Label, Technology, Description string
	Kind                               C4ContainerKind
	// Icon is a sprite name ("go", "postgresql", "globe", …); writers that
	// cannot draw icons ignore it.
	Icon string
}

// C4Relation is one arrow. Technology is empty for an actor's relations.
type C4Relation struct {
	From, To, Label, Technology string
}

const (
	eventSystemID = "Event_System"
	eventQueueID  = "Event_Queue"
)

// c4Builder holds the state of one BuildC4 call.
type c4Builder struct {
	doc  *model.CraftDoc
	its  []interaction.Interaction
	opts C4Options

	hasFocus   bool
	focused    map[string]bool   // service name → internal
	ctxSvc     map[string]string // context key → service
	containers map[string]C4Container
	// entries maps an entry context key (reached first by a user) to its
	// service; in transparent mode every context a user scenario touches.
	entries map[string]string

	actors    map[string]bool
	relations []C4Relation
}

// BuildC4 lays out a C4 container diagram.
func BuildC4(doc *model.CraftDoc, its []interaction.Interaction, opts C4Options) C4 {
	b := &c4Builder{
		doc: doc, its: its, opts: opts,
		focused: map[string]bool{}, ctxSvc: map[string]string{},
		containers: map[string]C4Container{}, entries: map[string]string{},
		actors: map[string]bool{},
	}
	b.indexServices()
	b.findEntries()
	b.findActors()

	var c4 C4
	mode := "transparent"
	if opts.Boundaries {
		mode = "boundaries"
	}
	c4.Title = fmt.Sprintf("Container Diagram - Architecture (%s mode)", mode)

	var systems []C4System
	for _, svc := range doc.Services {
		systems = append(systems, b.serviceSystem(svc))
	}
	var presentation, gateway *C4System
	if b.wantsLayers() {
		presentation = b.layerSystem("Presentation", "Presentation", func(a model.ArchBlock) []model.Component { return a.Presentation }, inferPresentationTechnology)
		gateway = b.layerSystem("Gateway", "Gateway", func(a model.ArchBlock) []model.Component { return a.Gateway }, inferGatewayTechnology)
		for _, s := range []*C4System{presentation, gateway} {
			if s != nil {
				systems = append(systems, *s)
			}
		}
	}
	if b.wantsEventQueue() {
		q := C4Container{ID: eventQueueID, Label: eventQueueID, Technology: "Message Queue",
			Description: "Handles asynchronous event processing and routing", Kind: C4Queue, Icon: "list"}
		b.containers[q.ID] = q
		systems = append(systems, C4System{ID: eventSystemID, Label: eventSystemID, Containers: []C4Container{q}})
	}
	sort.SliceStable(systems, func(i, j int) bool {
		if systems[i].External != systems[j].External {
			return systems[i].External
		}
		return systems[i].Label < systems[j].Label
	})
	c4.Systems = systems

	b.relateActors(presentation, gateway)
	b.relateServices()
	b.relateEvents()
	c4.Relations = dedupeRelations(b.relations)

	for _, name := range sortedKeys(b.actors) {
		c4.Actors = append(c4.Actors, b.actor(name))
	}
	c4.Languages = b.languages()
	return c4
}

// indexServices records which service deploys each context and which
// services are in focus.
func (b *c4Builder) indexServices() {
	focusCtx := map[string]bool{}
	for _, c := range b.opts.FocusContexts {
		focusCtx[c] = true
	}
	for _, s := range b.opts.FocusServices {
		b.focused[s] = true
	}
	b.hasFocus = len(b.opts.FocusServices) > 0 || len(b.opts.FocusContexts) > 0
	for _, svc := range b.doc.Services {
		for i, ctx := range svc.Contexts {
			key := contextKey(svc, i)
			if _, taken := b.ctxSvc[key]; !taken {
				b.ctxSvc[key] = svc.Name
			}
			if focusCtx[ctx] || focusCtx[key] {
				b.focused[svc.Name] = true
			}
		}
	}
}

func contextKey(svc model.Service, i int) string {
	if i < len(svc.ContextIDs) && svc.ContextIDs[i] != "" {
		return svc.ContextIDs[i]
	}
	return svc.Contexts[i]
}

func (b *c4Builder) inFocus(service string) bool { return !b.hasFocus || b.focused[service] }

// userScenarios yields the scenarios a user (or anything in the actor slot of
// an external trigger) starts.
func (b *c4Builder) userScenarios(fn func(sc model.Scenario)) {
	for _, uc := range b.doc.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == lang.TriggerExternal && sc.Trigger.Actor != "" {
				fn(sc)
			}
		}
	}
}

// findEntries finds the contexts users reach: in boundaries mode the first
// context of each user scenario, otherwise every context it touches.
func (b *c4Builder) findEntries() {
	b.userScenarios(func(sc model.Scenario) {
		for _, a := range sc.Actions() {
			if a.Context == "" {
				continue
			}
			if svc := b.ctxSvc[key(a.Context, a.From)]; svc != "" {
				b.entries[key(a.Context, a.From)] = svc
			}
			if b.opts.Boundaries {
				return
			}
			if a.TargetContext != "" {
				if svc := b.ctxSvc[key(a.TargetContext, a.To)]; svc != "" {
					b.entries[key(a.TargetContext, a.To)] = svc
				}
			}
		}
	})
}

// findActors keeps the actors that trigger a scenario — under focus, only
// those whose scenario touches a focused service.
func (b *c4Builder) findActors() {
	for _, it := range b.its {
		if it.Kind != interaction.Trigger {
			continue
		}
		if b.hasFocus && !b.scenarioTouchesFocus(it.Scenario) {
			continue
		}
		b.actors[it.From.Key] = true
	}
}

func (b *c4Builder) scenarioTouchesFocus(scenario string) bool {
	for _, it := range b.its {
		if it.Scenario != scenario || it.Kind == interaction.Trigger {
			continue
		}
		for _, e := range []interaction.Endpoint{it.From, it.To} {
			if svc := b.ctxSvc[e.Key]; svc != "" && b.focused[svc] {
				return true
			}
		}
	}
	return false
}

func key(written string, p *model.Participant) string {
	if p != nil {
		return p.ID
	}
	return written
}

func (b *c4Builder) serviceSystem(svc model.Service) C4System {
	sys := C4System{ID: svc.Name, Label: svc.Name, External: !b.inFocus(svc.Name)}
	l, _ := lookupLanguage(svc.Language)
	tech := serviceTechnology(svc.Language)
	icon := l.sprite
	if svc.Language == "" {
		icon = ""
	} else if icon == "" {
		icon = "code"
	}
	if b.opts.Boundaries {
		for i, ctx := range svc.Contexts {
			c := C4Container{ID: contextKey(svc, i), Label: ctx, Technology: tech,
				Description: ctx + " [Context]", Kind: C4App, Icon: icon}
			b.containers[c.ID] = c
			sys.Groups = append(sys.Groups, C4Group{ID: svc.Name + "/" + ctx, Label: ctx + " Domain", Containers: []C4Container{c}})
		}
	} else if len(svc.Contexts) > 0 {
		c := C4Container{ID: appContainerID(svc.Name), Label: svc.Name + " Application", Technology: tech,
			Description: fmt.Sprintf("Core business logic for %s domains: %s", svc.Name, strings.Join(svc.Contexts, ", ")),
			Kind:        C4App, Icon: icon}
		b.containers[c.ID] = c
		sys.Containers = append(sys.Containers, c)
	}
	if b.opts.ShowDatabases {
		for _, store := range svc.DataStores {
			techDB := databaseTechnology(store)
			c := C4Container{ID: dbContainerID(svc.Name, store), Label: svc.Name + "_" + store, Technology: techDB,
				Description: fmt.Sprintf("Stores %s data", store), Kind: C4Database, Icon: databaseIcon(techDB)}
			b.containers[c.ID] = c
			sys.Containers = append(sys.Containers, c)
		}
	}
	return sys
}

func appContainerID(service string) string       { return "app:" + service }
func dbContainerID(service, store string) string { return "db:" + service + "/" + store }

// containerOf finds the container that draws an endpoint, if any.
func (b *c4Builder) containerOf(e interaction.Endpoint) string {
	if e.Kind == model.ParticipantDataStore {
		if _, ok := b.containers[dbContainerID(e.Service, e.Key)]; ok {
			return dbContainerID(e.Service, e.Key)
		}
		return ""
	}
	svc := b.ctxSvc[e.Key]
	if svc == "" {
		return ""
	}
	if _, ok := b.containers[e.Key]; ok && b.opts.Boundaries {
		return e.Key
	}
	if _, ok := b.containers[appContainerID(svc)]; ok {
		return appContainerID(svc)
	}
	return ""
}

func (b *c4Builder) serviceOf(e interaction.Endpoint) string {
	if e.Kind == model.ParticipantDataStore {
		return e.Service
	}
	return b.ctxSvc[e.Key]
}

func (b *c4Builder) hasLayers() bool {
	for _, a := range b.doc.Architectures {
		if len(a.Presentation) > 0 || len(a.Gateway) > 0 {
			return true
		}
	}
	return false
}

// wantsLayers: presentation and gateway layers are drawn when the model
// declares them and users reach a service in focus.
func (b *c4Builder) wantsLayers() bool {
	if !b.hasLayers() {
		return false
	}
	for _, svc := range b.entries {
		if b.inFocus(svc) {
			return true
		}
	}
	return false
}

func (b *c4Builder) layerSystem(id, label string, pick func(model.ArchBlock) []model.Component, tech func(model.Component) string) *C4System {
	sys := C4System{ID: id, Label: label}
	for _, a := range b.doc.Architectures {
		for _, comp := range pick(a) {
			name := comp.Name
			if comp.Type == model.ComponentTypeFlow && len(comp.Chain) > 0 {
				name = comp.Chain[len(comp.Chain)-1].Name
			}
			cid := "layer:" + id + "/" + name
			if _, dup := b.containers[cid]; dup {
				continue
			}
			c := C4Container{ID: cid, Label: name, Technology: tech(comp),
				Description: componentDescription(comp, label), Kind: C4App, Icon: layerIcon(name)}
			b.containers[cid] = c
			sys.Containers = append(sys.Containers, c)
		}
	}
	if len(sys.Containers) == 0 {
		return nil
	}
	return &sys
}

func layerIcon(name string) string {
	switch {
	case strings.Contains(name, "Presentation"):
		return "globe"
	case strings.Contains(name, "Gateway"):
		return "shield_alt"
	}
	return "code"
}

// wantsEventQueue: the queue is drawn when a service in focus publishes.
func (b *c4Builder) wantsEventQueue() bool {
	for _, it := range b.its {
		if it.Kind != interaction.Publish {
			continue
		}
		if !b.hasFocus {
			return true
		}
		if svc := b.serviceOf(it.From); svc != "" && b.focused[svc] {
			return true
		}
	}
	return false
}

func (b *c4Builder) relate(from, to, label, tech string) {
	if from != "" && to != "" {
		b.relations = append(b.relations, C4Relation{From: from, To: to, Label: label, Technology: tech})
	}
}

// relateActors connects users through the layers when they exist, otherwise
// straight to the first container each user scenario reaches.
func (b *c4Builder) relateActors(presentation, gateway *C4System) {
	if b.hasLayers() {
		if presentation != nil {
			for _, actor := range sortedKeys(b.actors) {
				for _, c := range presentation.Containers {
					b.relate(actor, c.ID, "Uses", "")
				}
			}
			if gateway != nil {
				for _, p := range presentation.Containers {
					for _, g := range gateway.Containers {
						b.relate(p.ID, g.ID, "API Requests", "HTTPS/REST")
					}
				}
			}
		}
		if gateway != nil {
			b.relateGateway(gateway)
		}
		return
	}
	seen := map[string]bool{}
	for _, it := range b.its {
		if it.Kind != interaction.Trigger || !b.actors[it.From.Key] || seen[it.Scenario] {
			continue
		}
		for _, other := range b.its {
			if other.Scenario != it.Scenario || other.Kind == interaction.Trigger {
				continue
			}
			if c := b.containerOf(other.From); c != "" && b.containers[c].Kind != C4Database {
				seen[it.Scenario] = true
				b.relate(it.From.Key, c, "Triggers", "")
				break
			}
		}
	}
}

func (b *c4Builder) relateGateway(gateway *C4System) {
	for _, g := range gateway.Containers {
		if b.opts.Boundaries {
			for _, ctx := range sortedKeys(b.entries) {
				if c, ok := b.containers[ctx]; ok && c.Kind != C4Database {
					b.relate(g.ID, ctx, fmt.Sprintf("Routes %s requests", c.Label), "HTTP/gRPC")
				}
			}
			continue
		}
		services := map[string]bool{}
		for _, svc := range b.entries {
			services[svc] = true
		}
		for _, svc := range sortedKeys(services) {
			if _, ok := b.containers[appContainerID(svc)]; ok {
				b.relate(g.ID, appContainerID(svc), "Routes business logic requests", "HTTP/gRPC")
			}
		}
	}
}

// relateServices draws calls and targeted returns between services, and the
// services' use of their data stores.
func (b *c4Builder) relateServices() {
	for _, it := range b.its {
		switch it.Kind {
		case interaction.Request:
			if it.To.Kind == model.ParticipantDataStore {
				if b.opts.Boundaries {
					b.relate(b.containerOf(it.From), b.containerOf(it.To), it.Label, "Database Query")
				}
				continue
			}
			from, to := b.serviceOf(it.From), b.serviceOf(it.To)
			if from != "" && to != "" && from != to {
				b.relate(b.containerOf(it.From), b.containerOf(it.To), it.Label, "Service API")
			}
		case interaction.Response:
			from, to := b.serviceOf(it.From), b.serviceOf(it.To)
			if it.Targeted && from != "" && to != "" && from != to {
				b.relate(b.containerOf(it.From), b.containerOf(it.To), it.Label, "Return Data")
			}
		}
	}
	if b.opts.ShowDatabases && !b.opts.Boundaries {
		for _, svc := range b.doc.Services {
			for _, store := range svc.DataStores {
				b.relate(b.containerOf(interaction.Endpoint{Key: firstContextKey(svc)}), dbContainerID(svc.Name, store), "Reads/Writes data", "Database Protocol")
			}
		}
	}
}

func firstContextKey(svc model.Service) string {
	if len(svc.Contexts) == 0 {
		return ""
	}
	return contextKey(svc, 0)
}

// relateEvents routes publications into the event queue and deliveries out.
func (b *c4Builder) relateEvents() {
	if _, ok := b.containers[eventQueueID]; !ok {
		return
	}
	for _, it := range b.its {
		if it.Kind == interaction.Publish {
			b.relate(b.containerOf(it.From), eventQueueID, it.Event, "Event Publishing")
		}
	}
	for _, it := range b.its {
		if it.Kind == interaction.Listen {
			b.relate(eventQueueID, b.containerOf(it.To), it.Event, "Event Delivery")
		}
	}
}

func dedupeRelations(rels []C4Relation) []C4Relation {
	seen := map[C4Relation]bool{}
	var out []C4Relation
	for _, r := range rels {
		k := r
		k.Technology = ""
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

func (b *c4Builder) actor(name string) C4Actor {
	a := C4Actor{ID: name, Label: name, Element: C4Person, Description: "External user"}
	for _, decl := range b.doc.Actors {
		if decl.Name != name {
			continue
		}
		switch decl.Type {
		case model.ActorTypeSystem:
			a.Element, a.Description = C4ExternalSystem, "External system"
		case model.ActorTypeService:
			a.Element, a.Description = C4ExternalSystem, "External service"
		}
		break
	}
	return a
}

func (b *c4Builder) languages() []string {
	seen := map[string]bool{}
	for _, svc := range b.doc.Services {
		if l, ok := lookupLanguage(svc.Language); ok {
			seen[l.sprite] = true
		}
	}
	return sortedKeys(seen)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func componentDescription(c model.Component, layer string) string {
	if c.Type == model.ComponentTypeFlow && len(c.Chain) > 0 {
		var names, mods []string
		for _, link := range c.Chain {
			names = append(names, link.Name)
			for _, m := range link.Modifiers {
				mods = append(mods, fmt.Sprintf("%s[%s]", link.Name, modifierText(m)))
			}
		}
		d := fmt.Sprintf("%s composed by: %s", layer, strings.Join(names, " → "))
		if len(mods) > 0 {
			d += " | Modifiers: " + strings.Join(mods, ", ")
		}
		return d
	}
	if len(c.Modifiers) == 0 {
		return ""
	}
	var mods []string
	for _, m := range c.Modifiers {
		mods = append(mods, modifierText(m))
	}
	return fmt.Sprintf(" [%s]", strings.Join(mods, ", "))
}

func modifierText(m model.ComponentModifier) string {
	if m.Value == "" {
		return m.Key
	}
	return m.Key + ":" + m.Value
}

func title(s string) string {
	r := []rune(s)
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

func inferPresentationTechnology(c model.Component) string {
	for _, m := range c.Modifiers {
		if m.Key == "framework" || m.Key == "tech" {
			return title(m.Value) + " Frontend"
		}
	}
	name := strings.ToLower(c.Name)
	switch {
	case strings.Contains(name, "react"):
		return "React Frontend"
	case strings.Contains(name, "vue"):
		return "Vue.js Frontend"
	case strings.Contains(name, "angular"):
		return "Angular Frontend"
	case strings.Contains(name, "mobile"):
		return "Mobile App"
	}
	return "Frontend Application"
}

func inferGatewayTechnology(c model.Component) string {
	for _, m := range c.Modifiers {
		if m.Key == "type" || m.Key == "tech" {
			return title(m.Value) + " Gateway"
		}
	}
	name := strings.ToLower(c.Name)
	switch {
	case strings.Contains(name, "nginx"):
		return "Nginx Gateway"
	case strings.Contains(name, "loadbalancer"):
		return "Load Balancer"
	case strings.Contains(name, "api"):
		return "API Gateway"
	}
	return "Gateway"
}

func databaseTechnology(store string) string {
	s := strings.ToLower(store)
	switch {
	case strings.Contains(s, "postgres") || strings.Contains(s, "pg"):
		return "PostgreSQL Database"
	case strings.Contains(s, "mysql"):
		return "MySQL Database"
	case strings.Contains(s, "redis") || strings.Contains(s, "cache"):
		return "Redis Cache"
	case strings.Contains(s, "mongo"):
		return "MongoDB Database"
	}
	return "Database"
}

func databaseIcon(tech string) string {
	t := strings.ToLower(tech)
	switch {
	case strings.Contains(t, "postgres"):
		return "postgresql"
	case strings.Contains(t, "mysql"):
		return "mysql"
	case strings.Contains(t, "redis"):
		return "redis"
	case strings.Contains(t, "mongo"):
		return "mongodb"
	}
	return "database"
}
