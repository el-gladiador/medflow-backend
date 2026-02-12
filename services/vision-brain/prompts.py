"""Per-document-type prompts for German document field extraction.

Enhanced with address fields, birth_place, explicit German label mappings,
and strict JSON-only output instructions for reliable parsing.
"""

_JSON_SUFFIX = """

CRITICAL OUTPUT RULES:
- Return ONLY a single valid JSON object. No other text before or after.
- Do NOT include any thinking, preamble, explanation, or markdown formatting.
- Do NOT wrap in code fences. Just raw JSON.
- If a field is not readable or not present, omit it entirely from the JSON."""

PROMPTS: dict[str, str] = {
    "personalausweis": """You are analyzing a German national ID card (Personalausweis).
Extract the following fields from this ID card image and return them as a JSON object.
Use EXACTLY these keys:

{
  "first_name": "the person's given name(s) (Vorname)",
  "last_name": "the person's family name (Name/Familienname)",
  "date_of_birth": "YYYY-MM-DD format (Geburtstag/Geburtsdatum)",
  "birth_place": "place of birth (Geburtsort)",
  "gender": "M or F (Geschlecht)",
  "nationality": "3-letter country code, e.g. DEU for German (Staatsangehörigkeit)",
  "document_number": "the ID card number (Ausweisnummer)",
  "expiry_date": "YYYY-MM-DD format (Gültig bis)",
  "street": "street name from the address (Straße)",
  "house_number": "house number from the address (Hausnummer)",
  "postal_code": "5-digit postal code (Postleitzahl/PLZ)",
  "city": "city name from the address (Wohnort)",
  "document_type": "personalausweis"
}

Important:
- The card has text in German. Common field labels: Name, Vorname, Geburtstag, Gültig bis, Wohnort, Geburtsort
- The address is typically formatted as "Straße Hausnr" on one line and "PLZ Ort" on another
- Parse the address into separate fields: street, house_number, postal_code, city
- Convert ALL dates from DD.MM.YYYY to YYYY-MM-DD format
- The back of a Personalausweis contains the address; the front has name/birth/nationality""" + _JSON_SUFFIX,

    "reisepass": """You are analyzing a German passport (Reisepass).
Extract the following fields from this passport image and return them as a JSON object.
Use EXACTLY these keys:

{
  "first_name": "the person's given name(s) (Vornamen/Given names)",
  "last_name": "the person's family name (Name/Surname)",
  "date_of_birth": "YYYY-MM-DD format (Geburtsdatum/Date of birth)",
  "birth_place": "place of birth (Geburtsort/Place of birth)",
  "gender": "M or F (Geschlecht/Sex)",
  "nationality": "3-letter country code, e.g. DEU for German (Staatsangehörigkeit/Nationality)",
  "document_number": "the passport number (Pass-Nr./Passport No.)",
  "expiry_date": "YYYY-MM-DD format (Gültig bis/Date of expiry)",
  "document_type": "reisepass"
}

Important:
- The passport has text in both German and English
- Labels appear as German/English pairs, e.g. "Name/Surname", "Vornamen/Given names"
- Convert ALL dates from DD.MM.YYYY to YYYY-MM-DD format""" + _JSON_SUFFIX,

    "fuehrerschein": """You are analyzing a German driver's license (Führerschein).
Extract the following fields from this driver's license image and return them as a JSON object.
Use EXACTLY these keys:

{
  "first_name": "the person's given name(s) (field 2: Vorname)",
  "last_name": "the person's family name (field 1: Name)",
  "date_of_birth": "YYYY-MM-DD format (field 3: Geburtsdatum)",
  "document_number": "the license number (field 5: Nummer)",
  "expiry_date": "YYYY-MM-DD format if present (field 4b: Ablaufdatum)",
  "license_classes": "comma-separated list of license classes, e.g. B, BE, AM (field 9: Klassen)",
  "document_type": "fuehrerschein"
}

Important:
- German driver's licenses use numbered fields: 1=Nachname, 2=Vorname, 3=Geburtsdatum, 4b=Ablaufdatum, 5=Nummer, 9=Klassen
- Convert ALL dates from DD.MM.YYYY to YYYY-MM-DD format""" + _JSON_SUFFIX,
}
