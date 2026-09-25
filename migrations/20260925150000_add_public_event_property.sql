-- +goose Up
-- A public event may identify a property (fastighet) instead of, or besides,
-- an organisation and a workplace. The first source that needs it is
-- Stockholm's supervision of lifts and other motorised building equipment,
-- whose primary identity is the property designation. The municipality code
-- is the four-digit Swedish kommunkod ("0180" = Stockholm); the designation
-- is kept as the source wrote it; the address is optional source wording.
ALTER TABLE public_events
    ADD COLUMN property_municipality_code text,
    ADD COLUMN property_designation       text,
    ADD COLUMN property_address           text;

-- Either no property at all, or a municipality code with a designation;
-- an address never stands alone.
ALTER TABLE public_events
    ADD CONSTRAINT public_events_property_check CHECK (
        (property_municipality_code IS NULL AND property_designation IS NULL AND property_address IS NULL)
        OR (property_municipality_code ~ '^[0-9]{4}$' AND property_designation <> '')
    );

-- Lookup by property.
CREATE INDEX public_events_property_idx
    ON public_events (property_municipality_code, property_designation)
    WHERE property_designation IS NOT NULL;

-- +goose Down
DROP INDEX public_events_property_idx;
ALTER TABLE public_events DROP CONSTRAINT public_events_property_check;
ALTER TABLE public_events
    DROP COLUMN property_address,
    DROP COLUMN property_designation,
    DROP COLUMN property_municipality_code;
