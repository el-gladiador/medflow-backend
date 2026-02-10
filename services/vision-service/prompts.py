"""Per-document-type prompts for Ollama vision extraction."""

PROMPTS: dict[str, str] = {
    "personalausweis": """You are analyzing a German national ID card (Personalausweis).
Extract the following fields from this ID card image and return them as a JSON object.
Use EXACTLY these keys:

{
  "first_name": "the person's given name(s)",
  "last_name": "the person's family name",
  "date_of_birth": "YYYY-MM-DD format",
  "gender": "M or F",
  "nationality": "3-letter country code, e.g. DEU for German",
  "document_number": "the ID card number",
  "expiry_date": "YYYY-MM-DD format",
  "document_type": "personalausweis"
}

Important:
- The card has text in German. Field labels: Name, Vorname, Geburtstag, Gültig bis, etc.
- Dates should be converted to YYYY-MM-DD format
- If a field is not readable, omit it from the JSON
- Return ONLY valid JSON, no other text""",

    "reisepass": """You are analyzing a German passport (Reisepass).
Extract the following fields from this passport image and return them as a JSON object.
Use EXACTLY these keys:

{
  "first_name": "the person's given name(s)",
  "last_name": "the person's family name",
  "date_of_birth": "YYYY-MM-DD format",
  "gender": "M or F",
  "nationality": "3-letter country code, e.g. DEU for German",
  "document_number": "the passport number",
  "expiry_date": "YYYY-MM-DD format",
  "document_type": "reisepass"
}

Important:
- The passport has text in German and English
- Dates should be converted to YYYY-MM-DD format
- If a field is not readable, omit it from the JSON
- Return ONLY valid JSON, no other text""",

    "fuehrerschein": """You are analyzing a German driver's license (Führerschein).
Extract the following fields from this driver's license image and return them as a JSON object.
Use EXACTLY these keys:

{
  "first_name": "the person's given name(s)",
  "last_name": "the person's family name",
  "date_of_birth": "YYYY-MM-DD format",
  "document_number": "the license number",
  "expiry_date": "YYYY-MM-DD format if present",
  "license_classes": "comma-separated list of license classes, e.g. B, BE, AM",
  "document_type": "fuehrerschein"
}

Important:
- German driver's licenses use numbered fields (1=Nachname, 2=Vorname, 3=Geburtsdatum, 5=Nummer, 9=Klassen, 4b=Ablaufdatum)
- Dates should be converted to YYYY-MM-DD format
- If a field is not readable, omit it from the JSON
- Return ONLY valid JSON, no other text""",
}
