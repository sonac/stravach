package tg

import (
	"fmt"
	"testing"
)

func TestMakeNamesListMessage(t *testing.T) {
	input := "Sweat Fest Under the Stars\nLegs Afire, Soul on Fire (More Like Legs on Fire)\nEvening Sprint to the Couch\nRun, Darkness, Repeat\nSlogging Through the Dusk\nEvening Miles, Morning Regret\nDinner was Good, Run was Bad\nDarkness, Sweat, and Tears (Not Really)\nSunset Slogging\nNight Owl's Sprint\nShin Splints and Street Lights\nI Ran, Therefore I Am (Slightly) Alive\nDusk Dash to Nowhere in Particular\nTwilight Trot to Dinner Time\nLegs of Despair, Heart of Gold (Not Really)"
	msg := makeNamesListMessage(input)

	expected := "*Select a number with new name:*\n\n1. Sweat Fest Under the Stars\n2. Legs Afire, Soul on Fire (More Like Legs on Fire)\n3. Evening Sprint to the Couch\n4. Run, Darkness, Repeat\n5. Slogging Through the Dusk\n6. Evening Miles, Morning Regret\n7. Dinner was Good, Run was Bad\n8. Darkness, Sweat, and Tears (Not Really)\n9. Sunset Slogging\n0. 🔄 Regenerate\nC. ✏️ Enter custom prompt"
	if msg != expected {
		t.Errorf("unexpected message output.\nGot:\n%s\nWant:\n%s", msg, expected)
	}
}

func TestMakeInlineKeyboardForNames(t *testing.T) {
	activityID := int64(12345)
	names := "Name1\n, Name2\n, Name3\n, Name4\n, Name5\n, Name6\n, Name7\n, Name8\n, Name9\n, Name10\n"

	keyboard := makeInlineKeyboardForNames(activityID, names)

	if len(keyboard) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(keyboard))
	}

	btnNum := 1
	for i := 0; i < 3; i++ {
		row := keyboard[i]
		if len(row) != 3 {
			t.Errorf("expected row %d to have 3 buttons, got %d", i, len(row))
		}
		for _, btn := range row {
			expectedText := fmt.Sprintf("%d", btnNum)
			expectedCallback := fmt.Sprintf("%s:%d:%d", callbackPrefixActivity, activityID, btnNum)
			if btn.Text != expectedText {
				t.Errorf("expected button text '%s', got '%s'", expectedText, btn.Text)
			}
			if btn.CallbackData != expectedCallback {
				t.Errorf("expected callback data '%s', got '%s'", expectedCallback, btn.CallbackData)
			}
			btnNum++
		}
	}

	lastRow := keyboard[3]
	if len(lastRow) != 2 {
		t.Errorf("expected last row to have 2 buttons, got %d", len(lastRow))
	}
	if lastRow[0].Text != "🔄 Regenerate" || lastRow[1].Text != "✏️ Custom" {
		t.Errorf("unexpected texts in last row: got %q, %q", lastRow[0].Text, lastRow[1].Text)
	}
	if lastRow[0].CallbackData != fmt.Sprintf("%s:%d:0", callbackPrefixActivity, activityID) {
		t.Errorf("unexpected callback data for Regenerate: %s", lastRow[0].CallbackData)
	}
	if lastRow[1].CallbackData != fmt.Sprintf("%s:%d:C", callbackPrefixActivity, activityID) {
		t.Errorf("unexpected callback data for Custom: %s", lastRow[1].CallbackData)
	}

	names = "A\n, B\n, C"
	keyboard = makeInlineKeyboardForNames(activityID, names)
	if len(keyboard) != 2 {
		t.Errorf("expected 2 rows for 3 names, got %d", len(keyboard))
	}
	if len(keyboard[0]) != 3 {
		t.Errorf("expected 1st row to have 3 buttons, got %d", len(keyboard[0]))
	}
	if keyboard[1][0].Text != "🔄 Regenerate" || keyboard[1][1].Text != "✏️ Custom" {
		t.Errorf("unexpected texts in last row for short names: got %q, %q", keyboard[1][0].Text, keyboard[1][1].Text)
	}
}

func TestCleanName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple name",
			input: "My Awesome Ride",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with leading/trailing spaces",
			input: "  My Awesome Ride  ",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with special characters",
			input: "My Ride!@#$%^&*()_+-={}|[]\\:\";'<>?,./",
			want:  "My Ride!&()_-\"'?,.",
		},
		{
			name:  "name with multiple spaces between words",
			input: "My    Awesome   Ride",
			want:  "My Awesome Ride",
		},
		{
			name:  "name with numbers",
			input: "Ride 123",
			want:  "Ride 123",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only special characters",
			input: "!@#$%^&*()",
			want:  "!&()",
		},
		{
			name:  "name with hyphens and underscores",
			input: "My_Activity-Ride",
			want:  "My_Activity-Ride",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanName(tt.input)
			if got != tt.want {
				t.Errorf("Telegram.cleanName() = %v, want %v", got, tt.want)
			}
		})
	}
}
