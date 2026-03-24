package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel builds a cartModel with no session/uid — safe for View/Update tests
// that don't trigger any HTTP commands.
func newTestModel() cartModel {
	return cartModel{}
}

// TestInitialModel verifies that initialModel sets the expected zero values.
func TestInitialModel(t *testing.T) {
	m := initialModel(nil, "uid-123")
	if m.sess != nil {
		t.Error("expected sess to be passed through")
	}
	if m.uid != "uid-123" {
		t.Errorf("expected uid uid-123, got %q", m.uid)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
	if m.items != nil {
		t.Errorf("expected items nil, got %v", m.items)
	}
	if m.busy {
		t.Error("expected busy false")
	}
	if m.confirmDelete {
		t.Error("expected confirmDelete false")
	}
	if m.errText != "" {
		t.Errorf("expected errText empty, got %q", m.errText)
	}
}

// TestView_EmptyCart checks that an empty, non-busy, no-error model shows the
// "(cart is empty)" placeholder.
func TestView_EmptyCart(t *testing.T) {
	m := newTestModel()
	out := m.View()
	if !strings.Contains(out, "(cart is empty)") {
		t.Errorf("expected '(cart is empty)' in output, got:\n%s", out)
	}
}

// TestView_WithItems verifies that item names and table headers appear in the output.
func TestView_WithItems(t *testing.T) {
	m := newTestModel()
	m.items = []cartLine{
		{productID: "1", quantity: 2, name: "Mleko 2%", unitPrice: "3.49"},
		{productID: "2", quantity: 1, name: "Chleb razowy", unitPrice: "6.99"},
	}
	out := m.View()
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected table header NAME, got:\n%s", out)
	}
	if !strings.Contains(out, "QTY") {
		t.Errorf("expected table header QTY, got:\n%s", out)
	}
	if !strings.Contains(out, "Mleko 2%") {
		t.Errorf("expected item name 'Mleko 2%%', got:\n%s", out)
	}
	if !strings.Contains(out, "Chleb razowy") {
		t.Errorf("expected item name 'Chleb razowy', got:\n%s", out)
	}
}

// TestView_Busy checks that a busy model shows "Loading...".
func TestView_Busy(t *testing.T) {
	m := newTestModel()
	m.busy = true
	out := m.View()
	if !strings.Contains(out, "Loading...") {
		t.Errorf("expected 'Loading...' in output, got:\n%s", out)
	}
}

// TestView_Error checks that an error message surfaces with the "Error:" prefix.
func TestView_Error(t *testing.T) {
	m := newTestModel()
	m.errText = "network timeout"
	out := m.View()
	if !strings.Contains(out, "Error:") {
		t.Errorf("expected 'Error:' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "network timeout") {
		t.Errorf("expected error text in output, got:\n%s", out)
	}
}

// TestView_ConfirmDelete checks that confirmDelete=true surfaces the confirmation prompt.
func TestView_ConfirmDelete(t *testing.T) {
	m := newTestModel()
	m.items = []cartLine{{productID: "1", quantity: 1, name: "Item"}}
	m.confirmDelete = true
	out := m.View()
	if !strings.Contains(out, "Confirm delete") {
		t.Errorf("expected 'Confirm delete' in output, got:\n%s", out)
	}
}

// TestUpdate_NavigationKeys verifies cursor movement via down/up keys.
func TestUpdate_NavigationKeys(t *testing.T) {
	m := newTestModel()
	m.items = []cartLine{
		{productID: "1", name: "A"},
		{productID: "2", name: "B"},
		{productID: "3", name: "C"},
	}

	// Move down once.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := m2.(cartModel)
	if got.cursor != 1 {
		t.Errorf("after down: expected cursor 1, got %d", got.cursor)
	}

	// Move up again.
	m3, _ := got.Update(tea.KeyMsg{Type: tea.KeyUp})
	got2 := m3.(cartModel)
	if got2.cursor != 0 {
		t.Errorf("after up: expected cursor 0, got %d", got2.cursor)
	}
}

// TestUpdate_QuitKey verifies that pressing "q" returns tea.Quit.
func TestUpdate_QuitKey(t *testing.T) {
	m := newTestModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	// Execute the command and check it returns a QuitMsg.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestUpdate_DeleteConfirmFlow verifies the d → n flow for the delete-confirmation state.
func TestUpdate_DeleteConfirmFlow(t *testing.T) {
	m := newTestModel()
	m.items = []cartLine{{productID: "1", quantity: 1, name: "Item"}}

	// Press "d" — should set confirmDelete=true.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	after := m2.(cartModel)
	if !after.confirmDelete {
		t.Error("expected confirmDelete=true after 'd'")
	}

	// Press "n" — should cancel and reset confirmDelete.
	m3, _ := after.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	cancelled := m3.(cartModel)
	if cancelled.confirmDelete {
		t.Error("expected confirmDelete=false after 'n'")
	}
}

// TestUpdate_CartDataMsg verifies that receiving a cartDataMsg updates items and clears busy.
func TestUpdate_CartDataMsg(t *testing.T) {
	m := newTestModel()
	m.busy = true

	lines := []cartLine{
		{productID: "10", quantity: 3, name: "Jogurt naturalny", unitPrice: "2.29"},
		{productID: "11", quantity: 1, name: "Maslo extra", unitPrice: "7.49"},
	}

	m2, _ := m.Update(cartDataMsg{lines: lines})
	got := m2.(cartModel)

	if got.busy {
		t.Error("expected busy=false after cartDataMsg")
	}
	if len(got.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got.items))
	}
	if got.items[0].name != "Jogurt naturalny" {
		t.Errorf("unexpected first item name: %q", got.items[0].name)
	}
	if got.items[1].productID != "11" {
		t.Errorf("unexpected second item productID: %q", got.items[1].productID)
	}
	if got.errText != "" {
		t.Errorf("expected errText empty, got %q", got.errText)
	}
}

// TestUpdate_CartDataMsg_Error verifies that an error in cartDataMsg sets errText.
func TestUpdate_CartDataMsg_Error(t *testing.T) {
	m := newTestModel()
	m.busy = true

	m2, _ := m.Update(cartDataMsg{err: errForTest("api error")})
	got := m2.(cartModel)

	if got.busy {
		t.Error("expected busy=false")
	}
	if got.errText != "api error" {
		t.Errorf("expected errText 'api error', got %q", got.errText)
	}
}

// errForTest is a minimal error value for use in tests.
type errForTest string

func (e errForTest) Error() string { return string(e) }
