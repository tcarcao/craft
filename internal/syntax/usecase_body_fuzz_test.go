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
				if a, ok := st.Action(); ok {
					_, _, _ = a.SubjectName(), a.TargetName(), a.OpText()
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
