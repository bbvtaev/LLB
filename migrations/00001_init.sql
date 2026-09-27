-- +goose Up
CREATE TABLE words (
    id               bigserial PRIMARY KEY,
    language         text        NOT NULL,
    word             text        NOT NULL,
    translation      text        NOT NULL,
    note             text,
    status           text        NOT NULL DEFAULT 'new'
                     CHECK (status IN ('new', 'repeat', 'good', 'fluent')),
    streak           int         NOT NULL DEFAULT 0, -- fluent answers in a row
    due_at           timestamptz NOT NULL DEFAULT now(),
    last_reviewed_at timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX words_language_word_key ON words (language, lower(word));
CREATE INDEX words_due_idx ON words (language, due_at);

CREATE TABLE groups (
    id   bigserial PRIMARY KEY,
    name text NOT NULL UNIQUE
);

CREATE TABLE word_groups (
    word_id  bigint NOT NULL REFERENCES words (id) ON DELETE CASCADE,
    group_id bigint NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    PRIMARY KEY (word_id, group_id)
);

CREATE TABLE settings (
    key   text PRIMARY KEY,
    value text NOT NULL
);

-- add_word inserts a word (or updates its translation/note if it exists)
-- and attaches it to the given groups. Used by the bot and by data migrations:
--   SELECT add_word('en', 'apple', 'яблоко', 'an apple a day', ARRAY['еда']);
-- +goose StatementBegin
CREATE FUNCTION add_word(
    p_language    text,
    p_word        text,
    p_translation text,
    p_note        text   DEFAULT NULL,
    p_groups      text[] DEFAULT '{}'
) RETURNS bigint AS $$
DECLARE
    v_word_id  bigint;
    v_group_id bigint;
    v_group    text;
BEGIN
    INSERT INTO words (language, word, translation, note)
    VALUES (lower(p_language), p_word, p_translation, NULLIF(p_note, ''))
    ON CONFLICT (language, lower(word)) DO UPDATE
        SET translation = EXCLUDED.translation,
            note        = COALESCE(EXCLUDED.note, words.note)
    RETURNING id INTO v_word_id;

    FOREACH v_group IN ARRAY COALESCE(p_groups, '{}') LOOP
        INSERT INTO groups (name) VALUES (lower(v_group))
        ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
        RETURNING id INTO v_group_id;

        INSERT INTO word_groups (word_id, group_id) VALUES (v_word_id, v_group_id)
        ON CONFLICT DO NOTHING;
    END LOOP;

    RETURN v_word_id;
END
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION add_word;
DROP TABLE settings;
DROP TABLE word_groups;
DROP TABLE groups;
DROP TABLE words;
