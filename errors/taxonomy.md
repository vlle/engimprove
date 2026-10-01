# Taxonomy

The closed list of `category` values for `errors.jsonl`. A new category is added only here,
and only when none of the existing ones fits. Never rename a category — it breaks history.

## kind: grammar

| category | what it is | typical example |
|---|---|---|
| `articles` | a / an / the / zero article | ~~I opened PR~~ → I opened the PR |
| `prepositions` | preposition of place, time, direction | ~~in Monday~~ → on Monday |
| `tense-aspect` | tense and aspect choice | ~~I work here since 2021~~ → I have worked here since 2021 |
| `subject-verb-agreement` | subject–verb agreement | ~~the data are corrupted~~ / ~~he do~~ → he does |
| `plurals-countability` | number, countability | ~~informations~~ → information |
| `word-order` | word order, adverb placement | ~~I know what is it~~ → I know what it is |
| `modals` | can / could / should / must / have to | ~~must to~~ → must |
| `conditionals` | conditional sentences | ~~if it will fail~~ → if it fails |
| `pronouns-reference` | pronouns and what they refer to | a dangling it / they |
| `infinitive-gerund` | to do vs doing after a verb | ~~suggest to add~~ → suggest adding |
| `comparatives` | degrees of comparison | ~~more faster~~ → faster |
| `passive-voice-form` | a broken passive form | ~~it was happened~~ → it happened |
| `questions-negation` | questions, negation, auxiliaries | ~~I not agree~~ → I do not agree |
| `relative-clauses` | which / that / who, commas in them | ~~the service what fails~~ → the service that fails |
| `coordination` | conjunctions in lists, parallel structure | ~~A, B, C~~ → A, B, and C |
| `missing-head-noun` | a modifier left without its noun | ~~an excellent peer-to-peer~~ → an excellent peer-to-peer school |
| `dangling-modifier` | a participial phrase attached to the wrong subject | ~~My profile is Go, working in teams~~ → My profile is Go, and I work in teams |

## kind: spelling

| category | what it is |
|---|---|
| `spelling` | typos, British/American spelling mixed |

## kind: punctuation

| category | what it is | typical example |
|---|---|---|
| `commas` | commas, incl. the habit of one before that | ~~I think, that~~ → I think that |
| `apostrophes` | its/it's, possessives | ~~the teams decision~~ → the team's decision |
| `capitalization` | capital letters | ~~i~~ → I; service names |
| `hyphenation` | hyphen in compound modifiers | ~~long running job~~ → long-running job |
| `sentence-boundaries` | run-ons, splices, fragments | two sentences glued with a comma |

## kind: lexical (not applied to the text, a suggestion only)

| category | what it is | typical example |
|---|---|---|
| `word-choice` | the word is wrong in meaning | ~~decide the problem~~ → solve the problem |
| `collocation` | words do not go together | ~~make a research~~ → do research |
| `calque` | a calque from the native language | ~~I have a question to you~~ → I have a question for you |
| `false-friend` | a translator's false friend | ~~actual~~ instead of current |
| `phrasal-verbs` | the wrong or a stray phrasal verb | ~~discuss about~~ → discuss |
| `preposition-collocation` | a preposition fixed by the word | ~~depends from~~ → depends on |

## kind: style (not applied to the text, a suggestion only)

| category | what it is |
|---|---|
| `wordiness` | long where short would do |
| `register` | too formal / too casual for the context |
| `hedging` | extra softeners, or a lack of them |
| `redundancy` | saying the same thing twice |
| `clarity` | ambiguity, a heavy construction |
| `paragraphing` | structure: a wall of text, a missing takeaway |
| `number-format` | a digit where prose spells the number out (1 month → one month) |
