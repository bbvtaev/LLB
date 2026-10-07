-- +goose Up
-- Put the cards from 00002 into the "self" group. Only links existing words to the
-- group, so translations edited through the bot stay as they are.
INSERT INTO groups (name) VALUES ('self') ON CONFLICT (name) DO NOTHING;

INSERT INTO word_groups (word_id, group_id)
SELECT w.id, g.id
FROM words w
JOIN groups g ON g.name = 'self'
JOIN (VALUES
    ('occasionally'),
    ('slightly'),
    ('fossilize'),
    ('omit'),
    ('copula'),
    ('rigid'),
    ('collocations'),
    ('simultaneously'),
    ('explicit'),
    ('what time would be most *convenient* for you?'),
    ('*crumbles* in dust'),
    ('there''s some *pros* (*upsides*)'),
    ('new businesses *springing up* all over the place'),
    ('heavily fined for *littering*'),
    ('smth *grinds to a halt*'),
    ('neighborhood *gentrification*'),
    ('somewhat *neglected* and *derelict* buildings'),
    ('miles of blocks *crammed*'),
    ('that''s a really *vibrant* place'),
    ('to strike a nerve'),
    ('I was *frustrated*'),
    ('restrictiveness'),
    ('*foreseeable* man'),
    ('he was already *deceased*'),
    ('this is a *viable* solution'),
    ('this is a *feasible* solution'),
    ('*noble* person'),
    ('yeah, *undoubtedly*'),
    ('you made that pretty *evident*'),
    ('*sequential* numbers'),
    ('*fiddly* logic'),
    ('it does have *precedence* over you'),
    ('*adjacent* elements'),
    ('fill your boots'),
    ('that''s just *fabulous*'),
    ('I''ll take a *nappy-poo*'),
    ('likewise'),
    ('what you trying to *convey*?'),
    ('I can make an *assumption*'),
    ('the prices went through the roof'),
    ('the *ceiling* is leaking'),
    ('the *gloss* is better'),
    ('*succumb* to the *urge*'),
    ('*purge* cities from unbelievers')
) AS self_words (word) ON lower(self_words.word) = lower(w.word)
WHERE w.language = 'en'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM word_groups WHERE group_id = (SELECT id FROM groups WHERE name = 'self');
