package syntax

import "testing"

// FuzzParseUseCaseBody wraps random bodies in a use case (or parses them as a
// whole file) and checks the parser never panics and the tree reproduces its
// source (checkTreeText panics under test). It also walks the typed views of
// the journey-semantics constructs, so a view that indexes past a recovered,
// half-built node panics here rather than in the LSP.
func FuzzParseUseCaseBody(f *testing.F) {
	for _, body := range []string{
		"    when Seeker opens an advert\n        frontendplatform asks graphql for the advert  [ad]\n        par \"field resolvers\"\n            graphql asks atlas for a  [x]\n            graphql asks atlas for b  [y]\n        loop \"for each image\"\n            graphql asks media to sign  [signUrl]\n",
		"    when Seeker opens an advert\n        par \"the advert\"\n            graphql asks atlas for it\n        and \"the statistics\"\n            graphql asks stats for it\n            note \"cached\"\n",
		"    finding \"nothing retries\"\n    when Seller pays an invoice\n        a asks b to pay  [payInvoice]\n        note \"session forwarded\"\n",
		"    when graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n",
		"    when smsdispatch listens store.sms_queue.inserted cron \"* 10 * * 1-5\"\n        smsdispatch claims the rows\n",
		"    when invoicing listens store.sms_queue every \"5m\"\n        invoicing notifies store.sms_queue.updated\n",
		// v2.24: a listens roster (spec §4.1).
		"    when seshat listens sns.mercury-advert-state-changed, sns.mercury-vas-state-changed,\n            sns.mercury-message-received, sns.mercury-cooked\n        seshat writes one row per event\n",
		// v2.24: returns stream (§7.1).
		"    when Seller asks the assistant a question\n        frontendplatform asks graphql to stream the answer  [assistantStream]\n        graphql asks AiAssistant to stream the reply  [POST /api/v1/assistant/chat_streaming]\n        AiAssistant returns stream the reply as META, TOKEN and DONE events\n        graphql returns stream to frontendplatform each event to the client\n        frontendplatform appends every TOKEN delta to the message\n",
		// v2.24: operation lists (§8.1).
		"    when Partner saves an advert\n        app-manager asks AwsApiGateway to create the key and bind it to the plan  [CreateApiKey, CreateUsagePlanKey]\n        Partner asks ad/atlas-advert to save or delete the advert  [postAdvert, putAdvert, deleteAdvert]\n        a asks b for paths  [GET /nearest/paths/{latitude},{longitude}]\n",
		// v2.24: notifies one of (§8.3).
		"    when nmf_monetization listens stream.user_event_store\n        nmf_monetization reads the detail-type off the stored column\n        nmf_monetization notifies one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled,\n                nmfbus.CreateAutoSchedule, nmfbus.SubAccountBundleUpdated\n",
		// v2.24: SQL and ADDR (§9.1, first example).
		"    when cron \"0 18 * * *\"\n        obidoanalytics asks front for the city, region and developer graph  [SQL investment flat city region developer]\n        obidoanalytics asks ObjectStore to take the refreshed snapshot  [ADDR \"SNAPSHOT_DESTINATION\"]\n",
		// v2.24: inferred (§9.1, second example).
		"    when Seller resumes a selling draft\n        mobile asks graphql to open a selling chat session  [agenticSellingStart]\n        graphql asks ShaperAgent to initiate the chat  [inferred POST /api/v1/assistant/initiate_chat]\n        graphql asks ShaperAgent to ping  [inferred]\n",
		// v2.24: a dangling list comma at end of input.
		"    when s listens a.A,\n",
		// v2.25: //@ source cites (spec §14.1).
		"    when nmf_monetization listens nmfbus.AddOnPurchased\n        //@ re-monetization-framework functions/eb_email_sender/eb_email_sender.go:5 -- one rule, eight detail-types\n        nmf_monetization picks the mail resolver from the event type alone\n        //@ eb_email_sender.go:5-40\n        nmf_monetization asks Siren to send the purchase mail  [POST /api/v1/event]\n        //@ pkg/siren/siren.go:63-115 -- a 423 for a suppressed address counts as delivered\n",
		// v2.25: includes/continues links (spec §15.1).
		"    when Seller asks the assistant a question\n        includes \"Site session preamble\"\n        frontendplatform asks graphql to stream the answer  [assistantStream]\n",
		"    when Seller promotes an advert\n        frontendplatform asks graphql for the promotion offers  [promotionOffers]\n        alt \"pays now\"\n            continues \"Seller pays at the checkout\"\n",
		"    when S does\n        //@\n",
		"    when S does\n        includes\n",
		"    when S does\n        //@ :3,5-4 --\n        continues \"\"\n",
	} {
		f.Add(body, true)
	}
	f.Add("surface concierge {\n    callers: Advisor, unknown\n    spec: \"a.yaml\"\n    finding \"no auth\"\n    query the list  [getUserList]\n    command delete it  [deleteAccount]\n}\n", false)
	f.Fuzz(func(t *testing.T, body string, wrap bool) {
		src := body
		if wrap {
			src = "use_case \"F\" {\n" + body + "}\n"
		}
		g, _, _ := Parse(src)
		file := AsFile(Root(g))
		var walk func([]StepDecl)
		walk = func(steps []StepDecl) {
			for _, st := range steps {
				if n, ok := st.Note(); ok {
					_, _ = n.Kind(), n.Text()
					continue
				}
				if l, ok := st.Link(); ok {
					_, _ = l.Kind(), l.Title()
					continue
				}
				if a, ok := st.Action(); ok {
					_, _, _ = a.SubjectName(), a.TargetName(), a.OpText()
					_, _, _ = a.IsStream(), a.IsOneOf(), a.IsInferred()
					_ = a.EventItems()
					one, of := a.OneOfTokens()
					_, _ = one, of
					continue
				}
				if b, ok := st.Block(); ok {
					_ = b.IsFanOut()
					for _, br := range b.Branches() {
						_ = br.Label()
						walk(br.Steps())
					}
				}
			}
		}
		for _, uc := range file.UseCases() {
			for _, n := range uc.Notes() {
				_ = n.Text()
			}
			for _, sc := range uc.Scenarios() {
				tr := sc.Trigger()
				_, _, _ = tr.Kind(), tr.ContextName(), tr.EventRef()
				_ = tr.EventItems()
				walk(sc.Steps())
			}
		}
		for _, s := range file.Surfaces() {
			_, _, _ = s.Subject(), s.Callers(), s.Spec()
			for _, op := range s.Operations() {
				_, _, _ = op.Kind(), op.PhraseText(), op.OpText()
			}
		}
	})
}
