-- +goose Up
-- Cards imported from the "English cards" app. *Asterisks* mark the word being learned.
SELECT add_word('en', 'occasionally', 'sometimes, from time to time');
SELECT add_word('en', 'slightly', 'a little, a bit');
SELECT add_word('en', 'fossilize', 'lock in, to turn to stone');
SELECT add_word('en', 'omit', 'miss, skip');
SELECT add_word('en', 'copula', 'bunch, ligament, bundle, sheaf');
SELECT add_word('en', 'rigid', 'hard, tough, stiff, rough');
SELECT add_word('en', 'collocations', 'words in some scripted order');
SELECT add_word('en', 'simultaneously', 'at the same time');
SELECT add_word('en', 'explicit', 'obvious, accurate');
SELECT add_word('en', 'what time would be most *convenient* for you?', 'comfortable time, comfy');
SELECT add_word('en', '*crumbles* in dust', 'break, decay, disintegrate');
SELECT add_word('en', 'there''s some *pros* (*upsides*)', 'pluses');
SELECT add_word('en', 'new businesses *springing up* all over the place', 'appearing suddenly');
SELECT add_word('en', 'heavily fined for *littering*', 'litter pollution, throw rubbish/trash');
SELECT add_word('en', 'smth *grinds to a halt*', 'smth stops');
SELECT add_word('en', 'neighborhood *gentrification*', 'improve the hood');
SELECT add_word('en', 'somewhat *neglected* and *derelict* buildings', 'abandoned');
SELECT add_word('en', 'miles of blocks *crammed*', 'close and narrow');
SELECT add_word('en', 'that''s a really *vibrant* place', 'bright, lively place');
SELECT add_word('en', 'to strike a nerve', 'do smth lively badly etc.');
SELECT add_word('en', 'I was *frustrated*', 'disappointed');
SELECT add_word('en', 'restrictiveness', 'blocks, boundaries and constraints on freedom');
SELECT add_word('en', '*foreseeable* man', 'predictable man');
SELECT add_word('en', 'he was already *deceased*', 'dead');
SELECT add_word('en', 'this is a *viable* solution', 'that could work, working');
SELECT add_word('en', 'this is a *feasible* solution', 'workable, doable');
SELECT add_word('en', '*noble* person', 'honorable');
SELECT add_word('en', 'yeah, *undoubtedly*', 'definitely, evidently');
SELECT add_word('en', 'you made that pretty *evident*', 'obvious');
SELECT add_word('en', '*sequential* numbers', 'serial numbers');
SELECT add_word('en', '*fiddly* logic', 'exhausting, complicated, tricky');
SELECT add_word('en', 'it does have *precedence* over you', 'priority');
SELECT add_word('en', '*adjacent* elements', 'communicating, near');
SELECT add_word('en', 'fill your boots', 'don''t deny yourself nothing');
SELECT add_word('en', 'that''s just *fabulous*', 'incredible, tremendous');
SELECT add_word('en', 'I''ll take a *nappy-poo*', 'funny nap phrase');
SELECT add_word('en', 'likewise', 'also, mutual');
SELECT add_word('en', 'what you trying to *convey*?', 'transfer, inform');
SELECT add_word('en', 'I can make an *assumption*', 'supposition, guess');
SELECT add_word('en', 'the prices went through the roof', 'prices have skyrocketed');
SELECT add_word('en', 'the *ceiling* is leaking', 'roof is leaking');
SELECT add_word('en', 'the *gloss* is better', 'luster, polish, shine');
SELECT add_word('en', '*succumb* to the *urge*', 'give in to the impulse');
SELECT add_word('en', '*purge* cities from unbelievers', 'clean');

-- +goose Down
