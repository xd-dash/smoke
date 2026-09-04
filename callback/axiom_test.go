package callback

import "testing"

func TestParseAxiomCallback(t *testing.T) {
	dispatcher, err := Parse([]string{
		"axiom://events?domain=us-east-1.aws.edge.axiom.co&token-env=MY_AXIOM_TOKEN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispatcher.Empty() {
		t.Fatal("expected Axiom callback")
	}
}

func TestParseAxiomRequiresDataset(t *testing.T) {
	if _, err := Parse([]string{"axiom://"}); err == nil {
		t.Fatal("expected missing dataset error")
	}
}
