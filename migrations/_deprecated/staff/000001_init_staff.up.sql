-- Staff Service Schema
-- Handles employee records with German-specific fields

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Enums for German employment types
CREATE TYPE geschlecht AS ENUM ('maennlich', 'weiblich', 'divers');
CREATE TYPE familienstand AS ENUM ('ledig', 'verheiratet', 'geschieden', 'verwitwet', 'eingetragene_partnerschaft');
CREATE TYPE anstellungsart AS ENUM ('vollzeit', 'teilzeit', 'minijob', 'praktikant', 'werkstudent', 'aushilfe');
CREATE TYPE vertragsart AS ENUM ('unbefristet', 'befristet', 'probezeit');
CREATE TYPE arbeitszeitmodell AS ENUM ('festzeit', 'gleitzeit', 'vertrauensarbeitszeit', 'schichtarbeit');
CREATE TYPE krankenversicherungstyp AS ENUM ('gesetzlich', 'privat');
CREATE TYPE steuerklasse AS ENUM ('1', '2', '3', '4', '5', '6');
CREATE TYPE konfession AS ENUM ('keine', 'ev', 'rk', 'islam', 'sonstige');
CREATE TYPE ausweistyp AS ENUM ('personalausweis', 'reisepass');

-- Main employees table
CREATE TABLE employees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID, -- Link to user service (nullable for employees without system access)
    personalnummer VARCHAR(50) NOT NULL UNIQUE,

    -- Personal info
    vorname VARCHAR(100) NOT NULL,
    nachname VARCHAR(100) NOT NULL,
    profilbild TEXT,
    geburtsdatum DATE NOT NULL,
    geburtsort VARCHAR(100),
    geschlecht geschlecht NOT NULL,
    nationalitaet VARCHAR(100) NOT NULL DEFAULT 'Deutsch',
    familienstand familienstand,

    -- Employment info
    rolle VARCHAR(50) NOT NULL,
    abteilung VARCHAR(100),
    anstellungsart anstellungsart NOT NULL,
    vertragsart vertragsart NOT NULL,
    eintrittsdatum DATE NOT NULL,
    probezeitende DATE,
    befristungsende DATE,

    -- Working time
    wochenstunden DECIMAL(4,2) NOT NULL,
    urlaubstage INTEGER NOT NULL,
    arbeitszeitmodell arbeitszeitmodell DEFAULT 'festzeit',

    -- Metadata
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_employees_user_id ON employees(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_employees_personalnummer ON employees(personalnummer);
CREATE INDEX idx_employees_name ON employees(nachname, vorname);
CREATE INDEX idx_employees_active ON employees(is_active) WHERE deleted_at IS NULL;

-- Employee address
CREATE TABLE employee_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    strasse VARCHAR(200) NOT NULL,
    hausnummer VARCHAR(20) NOT NULL,
    plz VARCHAR(10) NOT NULL,
    ort VARCHAR(100) NOT NULL,
    land VARCHAR(100) DEFAULT 'Deutschland',
    zusatz VARCHAR(200),
    is_primary BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_employee_addresses_employee ON employee_addresses(employee_id);

-- Employee contact information
CREATE TABLE employee_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE UNIQUE,
    email_geschaeftlich VARCHAR(255) NOT NULL,
    email_privat VARCHAR(255),
    telefon_mobil VARCHAR(50),
    telefon_festnetz VARCHAR(50),
    -- Emergency contact
    notfallkontakt_name VARCHAR(200),
    notfallkontakt_beziehung VARCHAR(100),
    notfallkontakt_telefon VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_employee_contacts_employee ON employee_contacts(employee_id);

-- Employee financial data (sensitive - DSGVO)
CREATE TABLE employee_financials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE UNIQUE,
    -- Bank info
    kontoinhaber VARCHAR(200) NOT NULL,
    iban VARCHAR(34) NOT NULL,
    bic VARCHAR(11),
    bankname VARCHAR(100),
    -- Tax info
    steuer_id VARCHAR(11) NOT NULL,
    steuerklasse steuerklasse NOT NULL,
    konfession konfession DEFAULT 'keine',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_employee_financials_employee ON employee_financials(employee_id);

-- Employee social insurance
CREATE TABLE employee_social_insurance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE UNIQUE,
    sozialversicherungsnummer VARCHAR(12) NOT NULL,
    krankenkasse VARCHAR(200) NOT NULL,
    krankenversicherungstyp krankenversicherungstyp NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_employee_social_employee ON employee_social_insurance(employee_id);

-- Employee documents (IDs, permits)
CREATE TABLE employee_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    -- ID document
    ausweistyp ausweistyp,
    ausweisnummer VARCHAR(50),
    ausweis_ablauf DATE,
    -- Driver license classes
    fuehrerscheinklassen TEXT[], -- Array of classes: B, BE, A, etc.
    -- Work permit
    benoetigt_arbeitserlaubnis BOOLEAN DEFAULT FALSE,
    aufenthaltstitel_nummer VARCHAR(100),
    aufenthaltstitel_gueltig_bis DATE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_employee_documents_employee ON employee_documents(employee_id);

-- Uploaded files/documents
CREATE TABLE employee_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER,
    mime_type VARCHAR(100),
    category VARCHAR(50), -- 'contract', 'certificate', 'id_document', etc.
    uploaded_at TIMESTAMPTZ DEFAULT NOW(),
    uploaded_by UUID -- user_id from user service
);

CREATE INDEX idx_employee_files_employee ON employee_files(employee_id);
CREATE INDEX idx_employee_files_category ON employee_files(category);

-- Local cache for user data from user-service
CREATE TABLE user_cache (
    user_id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    role_name VARCHAR(50),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Transactional outbox for reliable event publishing
CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    routing_key VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retries INTEGER DEFAULT 0
);

CREATE INDEX idx_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_employees_updated_at
    BEFORE UPDATE ON employees
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_employee_addresses_updated_at
    BEFORE UPDATE ON employee_addresses
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_employee_contacts_updated_at
    BEFORE UPDATE ON employee_contacts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_employee_financials_updated_at
    BEFORE UPDATE ON employee_financials
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_employee_social_updated_at
    BEFORE UPDATE ON employee_social_insurance
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_employee_documents_updated_at
    BEFORE UPDATE ON employee_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
