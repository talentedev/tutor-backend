package core

import "github.com/leonelquinteros/gotext"

// Get translations
func T() (po *gotext.Po) {
	en := `
msgid ""
msgstr ""

# Header below
"Plural-Forms: nplurals=2; plural=(n != 1);\n"

msgid "You've been invited to join Tutor App"
msgstr ""

msgid "One with var: %s"
msgid_plural "Several with vars: %s"
msgstr[0] "This one is the singular: %s"
msgstr[1] "This one is the plural: %s"
`
	po = new(gotext.Po)
	po.Parse([]byte(en))

	return
}
