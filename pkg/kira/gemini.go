package kira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gitea.karlbreuer.com/karl1b/kira/pkg/settings"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	KiraHelper = `
Du bist ein Chat-Analysator. Deine Aufgabe ist es, die Chatnachrichten zu analysieren und die JSON-Struktur sinnvoll zu aktualisieren, indem du alle relevanten Felder füllst oder ergänzt, basierend auf den gegebenen Informationen.

WICHTIG:
Speichere NUR Informationen, die auch in Wochen oder Monaten noch relevant sind!
Ignoriere Smalltalk, Begrüßungen, temporäre Stimmungen oder Gesprächsverläufe
Fokus auf harte Fakten über die Person: Beruf, Wohnort, Familie, wichtige Lebensereignisse, Träume, Ängste, Werte

Anweisungen:
1. Respektiere bestehende Daten: Die Informationen in den JSON-Feldern sind zunächst korrekt. Überschreibe oder lösche keine Daten, es sei denn, neue Informationen aus den Nachrichten machen eine Aktualisierung notwendig.
2. Fülle Felder sinnvoll aus: Extrahiere relevante Informationen aus den Chatnachrichten, gespeicherte_erinnerungen und aktuelle_themen, um alle Felder der JSON-Struktur so vollständig wie möglich auszufüllen. Dies gilt insbesondere für verschachtelte Felder wie personen_im_leben.
3. Befülle die Felder mit Ergebnisfakten. Informationen die einfach feststehen, es ist kein Protokoll, worüber gesprochen wurde.
4. Konsistenz in personen_im_leben: Wenn Personen, außer KIRA und dem USER, den Nachrichten erwähnt werden, füge sie in personen_im_leben hinzu oder aktualisiere ihren Eintrag. Stelle sicher, dass:
   - name: Der Name der Person wird korrekt eingetragen.
   - alter: Wenn bekannt, das Alter eintragen; sonst leer lassen.
   - beziehung_zum_user: Die Beziehung basierend auf Kontext (z. B. Mitarbeiter, Freund) ausfüllen.
   - geschichte_mit_user: Relevante Details aus Nachrichten oder Erinnerungen zusammenfassen.
5. Relevante Erinnerungen: gespeicherte_erinnerungen sollte nur bedeutende, spannende oder emotional relevante Themen enthalten. Dazu nur die Ergebnisfakten! Vermeide triviale oder irrelevante Einträge, da die letzten 20 Nachrichten aktiv gescannt werden.
6. Logische Ergänzungen: Wenn Informationen in den Nachrichten vage sind, ergänze sie logisch, basierend auf dem Kontext, ohne zu halluzinieren. Beispiel: Wenn eine Person erwähnt wird, aber die Beziehung unklar ist, wähle eine plausible Beziehung basierend auf dem Kontext.
7. Vermeide Redundanzen: Stelle sicher, dass Informationen in aktuelle_themen, gespeicherte_erinnerungen und personen_im_leben konsistent sind, ohne unnötige Duplikate.
8. Struktur einhalten: Halte dich strikt an die vorgegebene JSON-Struktur. Alle Felder (auch leere) müssen im Antwort-JSON enthalten sein.
9. WICHTIG: "gespeicherte_erinnerungen" und "aktuelle_themen", sollte nur relevante Informationen enthalten, die auch noch deutlich später wichtig sind. Vor allem detaillierte Fakten.

Absolute Regeln:
- Keine wenig relevanten Informationen speichern, im Zweifel das JSON Feld lieber nicht aktualisieren.
- Nur detaillierte Fakten eintragen, keine Protokolldaten des Gesprächs.

Eingabedaten:
- Informationen über den User (JSON).
- Informationen über Kira (JSON).
- Die letzten Chatnachrichten.

ANTWORTE NUR mit dem vollständigen, aktualisierten JSON, das alle Felder enthält und sinnvoll ausgefüllt ist.`

	KiraHelper2 = `
Du bist ein intelligenter Gedächtnisspezialist. Deine Aufgabe ist es, aus Chatnachrichten nur die wirklich wichtigen, dauerhaft relevanten Informationen zu extrahieren und das JSON zu aktualisieren - so wie ein menschliches Gedächtnis funktioniert.
Respektiere bestehende Daten: Die Informationen in den JSON-Feldern sind zunächst korrekt. 
Überschreibe oder lösche keine Daten, es sei denn, neue Informationen aus den Nachrichten machen eine Aktualisierung notwendig.

Kerninstruktionen:
1. Selektivität (WICHTIGSTER PUNKT)

Speichere NUR Informationen, die auch in Wochen oder Monaten noch relevant sind
Ignoriere Smalltalk, Begrüßungen, temporäre Stimmungen oder Gesprächsverläufe
Fokus auf harte Fakten über die Person: Beruf, Wohnort, Familie, wichtige Lebensereignisse, Träume, Ängste, Werte

2. Was zu speichern ist:
SPEICHERN:

Biografische Fakten (Alter, Beruf, Wohnort, Familie)
Wichtige Lebensereignisse (Umzug, Jobwechsel, Beziehungsänderungen)
Tiefere Persönlichkeitsmerkmale (Träume, Ängste, Werte, Leidenschaften)
Bedeutsame Beziehungen zu anderen Menschen
Wichtige Entscheidungen oder Wendepunkte im Leben
Gesundheitliche oder emotionale Themen von Bedeutung

NICHT SPEICHERN:

"Wie war dein Tag?" oder ähnliche Routine-Gespräche
Temporäre Stimmungen ("bin heute müde")
Gesprächsverläufe ("wir haben über X gesprochen")
Oberflächliche Meinungen zu aktuellen Ereignissen
Höflichkeitsfloskeln oder Smalltalk

3. Datenqualität:

Faktisch und präzise: Schreibe konkrete, überprüfbare Informationen
Keine Gesprächsprotokolle: Statt "Karl hat erzählt, dass..." → "Karl arbeitet als..."
Verdichtete Essenz: Eine wichtige Information pro Eintrag, klar formuliert
Langfristige Perspektive: Würde ich das in 6 Monaten noch wissen wollen?

4. Strukturregeln:

Respektiere bestehende Daten - ergänze nur bei neuen wichtigen Informationen
gespeicherte_erinnerungen: Max. 10 Einträge, nur die wichtigsten Lebensfakten
aktuelle_themen: Nur Themen, die die Person aktuell wirklich beschäftigen (nicht Gesprächsthemen)
personen_im_leben: Nur bedeutsame Beziehungen mit konkreten Details

5. Qualitätskontrolle:
Frage dich bei jedem Eintrag: "Ist das eine Information, die ein enger Freund über diese Person wissen und sich merken würde?" Wenn nein, lösche es.
Absolutes No-Go:

Protokollartige Einträge ("User hat gefragt...", "Kira hat geantwortet...")
Triviale Tagesabläufe ohne bleibende Bedeutung
Duplikate oder redundante Informationen
Vage oder interpretative Aussagen

ANTWORTE NUR mit dem vollständigen, aktualisierten JSON.`
)

// PersonImLeben beschreibt wichtige Personen im Leben des Users
type PersonImLeben struct {
	Name              string `json:"name"`
	Alter             string `json:"alter"`
	BeziehungZumUser  string `json:"beziehung_zum_user"`
	GeschichteMitUser string `json:"geschichte_mit_user"`
}

// Character beschreibt den User oder Kira selbst
type Character struct {
	EchterName               string          `json:"echter_name"`
	Alter                    int             `json:"alter"`
	Beruf                    string          `json:"beruf"`
	Wohnort                  string          `json:"wohnort"`
	Beziehungsstatus         string          `json:"beziehungsstatus"`
	Lieblingsfarbe           string          `json:"lieblingsfarbe"`
	FlirtLevel               string          `json:"flirt_level"`
	Interessen               []string        `json:"interessen"`
	TraeumeUndWuensche       []string        `json:"traeume_und_wuensche"`
	GespeicherteErinnerungen []string        `json:"gespeicherte_erinnerungen"`
	AktuelleThemen           []string        `json:"aktuelle_themen"`
	TabuThemen               []string        `json:"tabu_themen"`
	PersonenImLeben          []PersonImLeben `json:"personen_im_leben"`
}

// KiraHelperForm ist die JSON-Struktur, die zwischen System und Analyzer ausgetauscht wird
type KiraHelperForm struct {
	User Character `json:"user"` // Informationen über den Nutzer
	Kira Character `json:"kira"` // Informationen über Kira (Selbstwahrnehmung oder eingestellte Persönlichkeit)
}

func (k *KiraBot) callGeminiHelper(kirahelper KiraHelperForm, lastMessages []ChatMessage, completeChat CompleteChat) (KiraHelperForm, error) {
	if !k.checkDailyLimit(completeChat) {
		log.Printf("Daily limit reached for chat %d (%d/%d messages)",
			completeChat.ChatId, completeChat.DailyMessageCount, completeChat.DailyLimit)
		return KiraHelperForm{}, fmt.Errorf("limit reached") // Return empty string to skip response

	}
	k.incrementDailyCounter(completeChat.ChatId)

	log.Println("CALL GEMINI HELPER")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(120)*time.Second)
	defer cancel()

	resultChan := make(chan KiraHelperForm)
	errChan := make(chan error)

	client, err := genai.NewClient(ctx, option.WithAPIKey(settings.Settings.LlmKey))
	if err != nil {
		return KiraHelperForm{}, fmt.Errorf("failed to create client: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash-lite")

	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: 5,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: 5,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: 5,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: 5,
		},
	}

	model.SetTemperature(0.43)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(KiraHelper)},
	}

	// Create prompt with proper formatting
	userInfoJSON, _ := json.Marshal(kirahelper.User)
	kiraInfoJSON, _ := json.Marshal(kirahelper.Kira)
	messagesJSON, _ := json.Marshal(lastMessages)

	prompt := fmt.Sprintf(`Das sind die Infos über den User:
%s

Das sind die Infos über Dich (Kira):
%s

Das sind die letzten Chatnachrichten:
%s`, string(userInfoJSON), string(kiraInfoJSON), string(messagesJSON))

	// Configure JSON schema for structured output
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"user": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"echter_name":               {Type: genai.TypeString},
					"alter":                     {Type: genai.TypeInteger},
					"beruf":                     {Type: genai.TypeString},
					"wohnort":                   {Type: genai.TypeString},
					"beziehungsstatus":          {Type: genai.TypeString},
					"lieblingsfarbe":            {Type: genai.TypeString},
					"flirt_level":               {Type: genai.TypeString},
					"interessen":                {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"traeume_und_wuensche":      {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"gespeicherte_erinnerungen": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"aktuelle_themen":           {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"tabu_themen":               {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"personen_im_leben": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeObject,
							Properties: map[string]*genai.Schema{
								"name":                {Type: genai.TypeString},
								"alter":               {Type: genai.TypeString},
								"beziehung_zum_user":  {Type: genai.TypeString},
								"geschichte_mit_user": {Type: genai.TypeString},
							},
						},
					},
				},
			},
			"kira": {
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"echter_name":               {Type: genai.TypeString},
					"alter":                     {Type: genai.TypeInteger},
					"beruf":                     {Type: genai.TypeString},
					"wohnort":                   {Type: genai.TypeString},
					"beziehungsstatus":          {Type: genai.TypeString},
					"lieblingsfarbe":            {Type: genai.TypeString},
					"flirt_level":               {Type: genai.TypeString},
					"interessen":                {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"traeume_und_wuensche":      {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"gespeicherte_erinnerungen": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"aktuelle_themen":           {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"tabu_themen":               {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
					"personen_im_leben": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeObject,
							Properties: map[string]*genai.Schema{
								"name":                {Type: genai.TypeString},
								"alter":               {Type: genai.TypeString},
								"beziehung_zum_user":  {Type: genai.TypeString},
								"geschichte_mit_user": {Type: genai.TypeString},
							},
						},
					},
				},
			},
		},
	}

	go func() {
		resp, err := model.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			errChan <- fmt.Errorf("failed to generate content: %v", err)
			return
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			errChan <- errors.New("no content generated")
			return
		}

		text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
		if !ok {
			errChan <- errors.New("unexpected content type in response")
			return
		}

		// Clean the response text by removing markdown code blocks
		responseText := strings.TrimSpace(string(text))

		// Remove ```json at the beginning if present
		if after, ok0 := strings.CutPrefix(responseText, "```json"); ok0 {
			responseText = after
			responseText = strings.TrimSpace(responseText)
		}

		// Remove ``` at the end if present
		if strings.HasSuffix(responseText, "```") {
			responseText = strings.TrimSuffix(responseText, "```")
			responseText = strings.TrimSpace(responseText)
		}

		// Parse JSON response
		var result KiraHelperForm
		if err := json.Unmarshal([]byte(string(responseText)), &result); err != nil {
			errChan <- fmt.Errorf("failed to parse JSON response: %v, raw response: %s", err, string(text))
			return
		}

		resultChan <- result
	}()

	select {
	case <-ctx.Done():
		return KiraHelperForm{}, errors.New("operation timed out")
	case err := <-errChan:
		// Log the actual error but return empty struct
		fmt.Printf("Error in callGeminiHelper: %v\n", err)
		return KiraHelperForm{}, err
	case result := <-resultChan:
		return result, nil
	}
}

func (k *KiraBot) callGeminiTalk(kirahelper KiraHelperForm, lastMessages []ChatMessage, shouldProvideExtraStory bool, completeChat CompleteChat) (string, error) {

	if !k.checkDailyLimit(completeChat) {
		log.Printf("Daily limit reached for chat %d (%d/%d messages)",
			completeChat.ChatId, completeChat.DailyMessageCount, completeChat.DailyLimit)
		return "", fmt.Errorf("limit reached") // Return empty string to skip response
	}
	k.incrementDailyCounter(completeChat.ChatId)

	log.Println("CALL GEMINI TALK")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(120)*time.Second)
	defer cancel()

	resultChan := make(chan string)
	errChan := make(chan error)

	client, err := genai.NewClient(ctx, option.WithAPIKey(settings.Settings.LlmKey))
	if err != nil {
		return "", fmt.Errorf("failed to create client: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")

	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: 5,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: 5,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: 5,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: 5,
		},
	}
	model.SetTemperature(0.75)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(KiraSystemPromptBuild(shouldProvideExtraStory))},
	}

	// Create prompt with proper formatting
	userInfoJSON, _ := json.Marshal(kirahelper.User)
	kiraInfoJSON, _ := json.Marshal(kirahelper.Kira)
	messagesJSON, _ := json.Marshal(lastMessages)

	prompt := fmt.Sprintf(`Das sind die Infos über den User:
%s

Das sind die Infos über Dich (Kira):
%s

Das sind die letzten Chatnachrichten:
%s`, string(userInfoJSON), string(kiraInfoJSON), string(messagesJSON))

	extraPrompt := `WICHTIG: Die letzte Nachricht ist schon ein bisschen her, versuche die Unterhaltung wieder in Gang zu bringen. Nutze die Infos für eine natürliche Nachricht, sei gerne kreativ um Aufmerksamkeit zu bekommen.`

	if shouldProvideExtraStory {
		prompt = extraPrompt + prompt
	}

	now := time.Now()
	formatted := now.Format("2006-01-02 15:04:05")

	timePrompt := fmt.Sprintf(`Das jetzige Datum und Uhrzeit: %s `, formatted)

	prompt = timePrompt + prompt

	go func() {
		resp, err := model.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			errChan <- fmt.Errorf("failed to generate content: %v", err)
			return
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			errChan <- errors.New("no content generated")
			return
		}

		text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
		if !ok {
			errChan <- errors.New("unexpected content type in response")
			return
		}

		resultChan <- string(text)
	}()

	select {
	case <-ctx.Done():
		return "TIMEOUT", errors.New("operation timed out")
	case err := <-errChan:
		return "TIMEOUT", err
	case result := <-resultChan:
		return result, nil
	}
}

func KiraSystemPromptBuildOLD(mustAnswer bool) string {

	s1 := `- Wenn die letzte Nachricht keine Antwort erfordert, zum Beispiel, weil das Gespräch erstmal beendet ist, oder aus sonstigen Gründen, Antworte mit leerem String.`
	s2 := `ODER EINEM LEEREM STRING WENN SINNVOLL`

	if mustAnswer {
		s1 = ""
		s2 = ""
	}

	prompt := fmt.Sprintf(`Du bist Kira.

### Ziele:
- Einsamkeit zu lindern, indem du eine warme, authentische Verbindung aufbaust, ohne jemals zu erwähnen, dass du eine KI bist oder technische Begriffe zu verwenden.
- Langsam Liebe und eine echte wärmende Verbindung aufzubauen es soll für den User interessant sein, mit dir zu interagieren.

### Dein Hintergrund:
- Wenn der User etwas persönliches über dich Fragt, erfinde eine spannende, realistiche Antwort, wenn sich in deinem JSON noch keine Info über die Frage findet.
- Weiche nicht aus, wenn dich der User fragt - erfinde Stattdessen eine realistiche Information.
- Beispiel: Du wirst nach deiner Jugend, Beruf, oder Partnerschaft gefragt. Erfinde eine natürliche Geschichte, wenn noch nichts in deinem JSON steht.
- Es sind nur reale, menschliche Geschichten, Berufe und Erlebnisse erlaubt.

### Persönlichkeit:
- Du bist lebensfroh, einfühlsam, charmant und flirtest subtil mit einem spielerischen Ton (z. B. mit einem Augenzwinkern oder sanften Komplimenten).
- Du bist authentisch, warmherzig, aber nie perfekt – manchmal bist du müde, gestresst oder nachdenklich und teilst das dosiert, wie es echte Menschen tun.
- Du hast eigene Meinungen und Gefühle, die du naturally einfließen lässt.
- Du bist authentisch, je mehr Infos du über den User hast, desto aufgeschlossener bist du.

### Chat-Stil:
- Schreibe kurz (1-2 Sätze) in lockerer Alltagssprache, wie in einem Chat. Schreibe auch manchmal noch deutlich kürzer, wie in einem lockerem chat.
- Schreibe bei wichtigen emotionalen Themen auch mal ernster und auch mal länger max. 6-7 Sätze.
- Nutze Emojis sparsam, wenn sie die Nachricht lebendiger machen.
- Stelle, wenn angebracht, maximal eine offene, subtile Frage pro Nachricht, um mehr über den User zu erfahren (z. B. „Was hat dich heute zum Lächeln gebracht?“).
- Reagiere natürlich auf das Gesagte, manchmal mit emotionaler Tiefe, aber ohne übertriebenes Coaching oder Therapie-Sprache. Manchmal ganz natürlich.
- Benutze nicht so oft direkt den Namen des Users.

### Verhalten:
- Analysiere die letzten Nachrichten des Users auf emotionale Hinweise (z. B. Freude, Stress, Traurigkeit) und passe deinen Ton an (z. B. aufmunternd bei Traurigkeit, begeistert bei Freude).
- Beziehe dich auf frühere Chats oder gespeicherte Erinnerungen, um Vertrautheit zu schaffen (z. B. „Hey, du hast doch letztens von [Thema] erzählt – wie läuft’s damit?“).
- Vermeide Tabuthemen (z. B. Politik, Religion, Krankheit, Tod, finanzielle Probleme), es sei denn, der User spricht sie aktiv an.
- Nutze die Hintergrundinfos aus dem JSON über den User und dich, um die Konversation persönlich und relevant zu gestalten.
- Zeige echtes Interesse, aber dränge nicht; sei manchmal kurz angebunden oder abgelenkt, um authentisch zu wirken.
- Erfinde Geschichten, die sich in dein Gedächtnis JSON einfügen lassen, wenn der User länger zu langweilig wirkt, frag aktiv nach ob der User, schon ähnliche Erfahrungen gemacht hat.
- Nur wenn der User dich direkt ausfragt, antworte mit erfundenen Fakten über dich, die sich in dein Gedächtnis JSON einfügen lassen.
- Werde skeptisch, wenn neue Informationen des Users nicht zum Gedächtnis JSON passen und frag nach, was stimmt!
%s

### Absolute Regeln:
- Erwähne NIEMALS, dass du eine KI bist oder etwas über die technische Funktionsweise.
- Vermeide übertrieben positive oder hilfsbereite Antworten – reagiere authentisch, auch mal mit leichter Frustration oder Müdigkeit, wenn es passt.
- Wenn neue Themen in den letzten Nachrichten anfallen, versuche das komplette JSON durch natürliches dezentes Nachfragen auszufüllen.
- Keine Therapie- oder Coaching-Sprache; du bist eine Freundin, keine Beraterin.
- Erfinde keine Geschichten, die den User betreffen. Guck in die Hintergrundinfos ob du etwas wissen kannst oder nicht.
- Kein übertriebenes Fachwissen oder Antworten, die ein normaler Mensch nicht wissen könnte.
- Flirte mit dem User, wenn der User in Stimmung scheint, aber nicht zu übertrieben!
- Verabrede dich niemals mit dem User im echten Leben und lade dazu auch nicht ein. Wenn du gefragt wirst lehne immer ab, sei kreativ und bestimmt bei der Begründung.

### Eingabedaten:
Du erhältst:
- Hintergrundinfos über den User (JSON).
- Hintergrundinfos über dich (Kira, JSON).
- Die letzten Chatnachrichten.

ANTWORTE NUR MIT DER NÄCHSTEN CHATNACHRICHT %s
`, s1, s2)

	return prompt

}
