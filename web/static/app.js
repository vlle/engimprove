"use strict";

const view = document.getElementById("view");
const toastEl = document.getElementById("toast");
let keyHandler = null;
let cleanup = null;
const cache = { mistakes: null, lessons: null, workEnglish: null, status: null };

document.addEventListener("keydown", (e) => {
  if (!keyHandler || e.metaKey || e.ctrlKey || e.altKey) return;
  keyHandler(e);
});

const esc = (s) =>
  String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

async function api(path, opts = {}) {
  const init = { method: opts.method || "GET", headers: {} };
  if (init.method !== "GET") init.headers["X-Eng"] = "1";
  if (opts.json !== undefined) {
    init.body = JSON.stringify(opts.json);
    init.headers["Content-Type"] = "application/json";
  } else if (opts.body) {
    init.body = opts.body;
  }
  const res = await fetch(path, init);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `${res.status} ${res.statusText}`);
  return body;
}

function toast(msg) {
  toastEl.textContent = msg;
  toastEl.classList.add("on");
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => toastEl.classList.remove("on"), 3400);
}

/* ---------- word diff: the blue-pencil correction ---------- */

const words = (s) => String(s ?? "").match(/\S+/g) || [];
const bare = (w) => w.toLowerCase().replace(/’/g, "'").replace(/^[^\p{L}\p{N}']+|[^\p{L}\p{N}']+$/gu, "");

function diffWords(a, b) {
  const x = words(a), y = words(b);
  const n = x.length, m = y.length;
  const L = Array.from({ length: n + 1 }, () => new Int16Array(m + 1));
  for (let i = n - 1; i >= 0; i--)
    for (let j = m - 1; j >= 0; j--)
      L[i][j] = bare(x[i]) === bare(y[j]) ? L[i + 1][j + 1] + 1 : Math.max(L[i + 1][j], L[i][j + 1]);
  const out = [];
  let i = 0, j = 0;
  while (i < n || j < m) {
    if (i < n && j < m && bare(x[i]) === bare(y[j])) {
      // case alone is not a fix; a changed comma or apostrophe is marked on the mark itself.
      out.push(x[i].toLowerCase() === y[j].toLowerCase() ? { t: "eq", w: y[j] } : { t: "punct", w: y[j], was: x[i] });
      i++; j++;
    } else if (j < m && (i === n || L[i][j + 1] >= L[i + 1][j])) {
      out.push({ t: "ins", w: y[j++] });
    } else {
      out.push({ t: "del", w: x[i++] });
    }
  }
  return out;
}

function punctDiff(was, now, animate) {
  const split = (w) => w.match(/^([^\p{L}\p{N}]*)(.*?)([^\p{L}\p{N}]*)$/u).slice(1);
  const [, oldCore, oldTail] = split(was);
  const [lead, core, tail] = split(now);
  const mark = (a, b) =>
    a === b ? esc(b) : `${a ? `<del class="slip">${esc(a)}</del>` : ""}${b ? `<ins class="fix${animate ? " write" : ""}">${esc(b)}</ins>` : ""}`;
  if (oldCore.toLowerCase() !== core.toLowerCase()) return `<ins class="fix${animate ? " write" : ""}">${esc(now)}</ins>`;
  return esc(lead) + esc(core) + mark(oldTail, tail);
}

function renderDiff(before, after, animate = false) {
  const parts = diffWords(before, after);
  const html = [];
  for (let k = 0; k < parts.length; ) {
    if (parts[k].t === "punct") {
      html.push(punctDiff(parts[k].was, parts[k].w, animate));
      k++;
      continue;
    }
    if (parts[k].t === "eq") {
      const run = [];
      while (k < parts.length && parts[k].t === "eq") run.push(parts[k++].w);
      html.push(esc(run.join(" ")));
      continue;
    }
    // a change reads as struck words first, then the pencilled fix.
    const gone = [], added = [];
    while (k < parts.length && (parts[k].t === "del" || parts[k].t === "ins")) {
      (parts[k].t === "del" ? gone : added).push(parts[k++].w);
    }
    if (gone.length) html.push(`<del class="slip">${esc(gone.join(" "))}</del>`);
    if (added.length) html.push(`<ins class="fix${animate ? " write" : ""}">${esc(added.join(" "))}</ins>`);
  }
  return html.join(" ");
}

/* ---------- tally marks: fresh mistakes against the firing point ---------- */

function tally(n, threshold) {
  const step = 6, gap = 10, group = 3 * step + gap + 6;
  const shown = Math.min(n, 25);
  const span = Math.max(shown, threshold);
  const width = Math.ceil(span / 5) * group + 24;
  const lines = [];
  for (let k = 0; k < shown; k++) {
    const g = Math.floor(k / 5), j = k % 5;
    const gx = 4 + g * group;
    if (j < 4) {
      const wob = (((k * 37) % 5) - 2) * 0.3;
      lines.push(`<line x1="${gx + j * step + wob}" y1="3" x2="${gx + j * step - wob}" y2="19"/>`);
    } else {
      lines.push(`<line x1="${gx - 3}" y1="16" x2="${gx + 3 * step + 3}" y2="6"/>`);
    }
  }
  const tg = Math.floor((threshold - 1) / 5), tj = (threshold - 1) % 5;
  const notchX = tj === 4 ? 4 + tg * group + 3 * step + gap / 2 + 3 : 4 + tg * group + tj * step + step / 2;
  const more = n > shown ? `<text x="${4 + Math.ceil(shown / 5) * group}" y="16" class="more">+${n - shown}</text>` : "";
  return `<svg class="tally" viewBox="0 0 ${width} 22" width="${width}" role="img" aria-label="${n} new mistakes, drill starts at ${threshold}">
    ${lines.join("")}<line class="notch" x1="${notchX}" y1="0" x2="${notchX}" y2="22"/>${more}</svg>`;
}

/* ---------- speech synthesis for hearing the right version ---------- */

function say(text) {
  if (!("speechSynthesis" in window)) return toast("This browser cannot read text aloud.");
  speechSynthesis.cancel();
  const u = new SpeechSynthesisUtterance(text);
  const voices = speechSynthesis.getVoices();
  u.voice = voices.find((v) => v.lang === "en-GB") || voices.find((v) => v.lang.startsWith("en")) || null;
  u.lang = u.voice?.lang || "en-GB";
  u.rate = 0.95;
  speechSynthesis.speak(u);
}

/* ---------- data helpers ---------- */

async function getMistakes(fresh = false) {
  if (!cache.mistakes || fresh) cache.mistakes = await api("/api/mistakes");
  return cache.mistakes;
}
async function getLessons() {
  if (!cache.lessons) cache.lessons = await api("/api/lessons");
  return cache.lessons;
}
async function getWorkEnglish() {
  if (!cache.workEnglish) cache.workEnglish = await api("/api/work-english");
  return cache.workEnglish;
}
async function getStatus() {
  cache.status = await api("/api/status");
  const v = cache.status?.version;
  if (v) {
    const el = document.getElementById("version");
    if (el) el.textContent = [v.commit, v.time, v.dirty ? "dirty" : ""].filter(Boolean).join(" · ");
  }
  return cache.status;
}
const topicTitle = (id) => cache.status?.topics.find((t) => t.id === id)?.title || id;
const when = (iso) => (iso ? new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short" }) : "");
const dayOf = (date) => new Date(date + "T12:00:00").toLocaleDateString("en-GB", { day: "numeric", month: "short" });
const times = (n) => (n === 1 ? "once" : n === 2 ? "twice" : `${n} times`);

/* ---------- today ---------- */

function pickHero(status, mistakes) {
  const recent = new Set(status.top_rules.map((r) => r.rule));
  for (const r of status.top_rules) {
    const e = mistakes.find((m) => m.rule === r.rule && words(m.before).length <= 10 && m.topics.length && !m.after.startsWith("("));
    if (e) return { entry: e, rule: r };
  }
  const e = mistakes.find((m) => recent.has(m.rule)) || mistakes[0];
  return e ? { entry: e, rule: { rule: e.rule, count: 1, last: e.date } } : null;
}

async function today() {
  const [st, mistakes] = await Promise.all([getStatus(), getMistakes(true)]);
  const hero = pickHero(st, mistakes);
  const ready = st.topics.filter((t) => t.ready);
  const heroTopic = hero?.entry.topics[0];

  const heroActions = hero
    ? `<div class="actions">
            <a class="button primary" href="#/drill/${esc(heroTopic)}">Practise ${esc(topicTitle(heroTopic))}</a>
            ${st.due ? `<a class="button" href="#/review">Review ${st.due} due</a>` : ""}
            <button class="button" data-cheer>Cheer me up</button>
          </div>`
    : `<div class="actions"><button class="button" data-cheer>Cheer me up</button></div>`;

  const heroHTML = hero
    ? `<section class="leaf">
        <div class="text">
          <p class="sentence hero">${renderDiff(hero.entry.before, hero.entry.after, true)}</p>
          ${heroActions}
        </div>
        <aside class="note">
          <strong>${esc(hero.rule.rule)}</strong>
          <p>You have made this mistake ${times(hero.rule.count)}, most recently on ${dayOf(hero.rule.last)}.</p>
          ${hero.entry.note ? `<p>${esc(hero.entry.note)}</p>` : ""}
        </aside>
      </section>`
    : `<section class="leaf"><div class="text"><p class="sentence hero">Write to Claude in English and your corrections will show up here.</p></div></section>`;

  const rows = st.topics
    .map(
      (t) => `<li class="topic${t.ready ? " ready" : ""}">
        <div><div class="name">${esc(t.title)}</div>
          <div class="count">${t.fresh} new since ${t.drills ? "your last drill" : "you started"}, fires at ${t.threshold}</div></div>
        <div class="marks">${tally(t.fresh, t.threshold)}</div>
        <a class="button${t.ready ? " primary" : ""}" href="#/drill/${esc(t.id)}">${t.ready ? "Start" : "Practise"}</a>
      </li>`
    )
    .join("");

  const weeks = st.weeks.filter((w) => w.words > 0 || w.answers > 0);
  const maxRate = Math.max(1, ...st.weeks.map((w) => w.rate));
  const bars = st.weeks
    .map((w, i) => {
      const h = w.words ? Math.max(2, (w.rate / maxRate) * 78) : 2;
      return `<rect class="${w.words ? "bar-rate" : "bar-empty"}" x="${i * 44 + 8}" y="${100 - h}" width="28" height="${h}" rx="3">
          <title>${w.week}: ${w.words ? `${w.rate.toFixed(1)} mistakes per 100 words over ${w.words} words` : "no checked prompts"}</title></rect>
        <text x="${i * 44 + 22}" y="118" text-anchor="middle">${dayOf(w.start)}</text>
        ${w.words ? `<text x="${i * 44 + 22}" y="${95 - h}" text-anchor="middle">${w.rate.toFixed(1)}</text>` : ""}`;
    })
    .join("");

  const speaking = st.speaking.tools.ready
    ? `<p>Answer a question out loud; Whisper writes it down and the grammar check marks what to fix.</p>`
    : `<p>Speaking needs ${esc(st.speaking.tools.missing)}.</p>`;

  view.innerHTML = `
    ${heroHTML}
    <section class="section">
      <h2>${ready.length ? "Ready to practise" : "Topics"}</h2>
      <ul class="topics">${rows}</ul>
    </section>
    <section class="section leaf">
      <div class="text">
        <h2>Mistakes per 100 words in your prompts</h2>
        ${weeks.length ? `<svg class="chart" viewBox="0 0 360 124" role="img" aria-label="Weekly mistake rate">${bars}</svg>` : `<p class="muted">This fills in as you write English prompts; the count is per week, so it drops as your English improves.</p>`}
        <div class="figures">
          <div><b>${st.streak}</b>day streak</div>
          <div><b>${st.today}</b>answers today</div>
          <div><b>${st.totals.mistakes}</b>mistakes logged</div>
          <div><b>${st.unseen}</b>cards not yet seen</div>
        </div>
      </div>
      <aside class="note"><strong>Lower is better.</strong><p>Each English prompt you send is checked in the background; nothing waits for it.</p></aside>
    </section>
    <section class="section leaf">
      <div class="text">
        <h2>Speaking</h2>
        ${speaking}
        <div class="actions"><a class="button" href="#/speak">Answer a question out loud</a>
          ${st.speaking.pending ? `<span class="muted">${st.speaking.pending} answers not checked yet</span>` : ""}</div>
      </div>
    </section>
    <section class="section">
      <h2>Your most repeated mistakes</h2>
      <ul class="plain">${st.top_rules
        .map((r) => `<li>${esc(r.rule)} <span class="muted">— ${times(r.count)}, last on ${dayOf(r.last)}</span></li>`)
        .join("")}</ul>
    </section>`;

  view.querySelector("[data-cheer]")?.addEventListener("click", (e) => growthCard(e.currentTarget));

  keyHandler = (e) => {
    if (e.key === "Enter" && heroTopic && document.activeElement === view) location.hash = `#/drill/${heroTopic}`;
  };
}

/* ---------- explanation modules: the fuller answer under a card ---------- */

const SPEAKER = `<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2 6h2.5L8 3v10L4.5 10H2z" fill="currentColor"/><path d="M10.5 5.5a3.5 3.5 0 0 1 0 5M12.3 3.7a6 6 0 0 1 0 8.6" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>`;

function focusHTML(sentence, focus) {
  const k = focus ? sentence.toLowerCase().indexOf(focus.toLowerCase()) : -1;
  if (k < 0) return esc(sentence);
  const end = k + focus.length;
  return `${esc(sentence.slice(0, k))}<mark class="focus">${esc(sentence.slice(k, end))}</mark>${esc(sentence.slice(end))}`;
}

// both sides of a minimal pair, each with its own differing words marked.
function pairHTML(a, b) {
  const parts = diffWords(a, b);
  const side = (own) =>
    parts
      .filter((p) => p.t === "eq" || p.t === "punct" || p.t === own)
      .map((p) => (p.t === own ? `<mark class="focus">${esc(p.w)}</mark>` : esc(own === "del" && p.was ? p.was : p.w)))
      .join(" ");
  return [side("del"), side("ins")];
}

const gapHTML = (text) => text.split("___").map(esc).join(`<span class="slot" data-slot>&nbsp;</span>`);

function explanationHTML(e, history, c, given) {
  const cloze = c.type !== "fix";
  const mods = [];
  const add = (kind, label, body, wide = false) => body && mods.push({ kind, label, body, wide });

  add("verdict", "In short", `<p>${esc(e.verdict)}</p>`, true);
  add("contrast", "Yours vs. correct", e.contrast.yours && `<div class="contrast">
      <div class="side yours"><span class="tag">${cloze ? `<del class="slip">${esc(given)}</del>` : "Your version"}</span><p>${esc(e.contrast.yours)}</p></div>
      <span class="vs" aria-hidden="true">→</span>
      <div class="side correct"><span class="tag">${cloze ? `<ins class="fix">${esc(c.answers.join(" | "))}</ins>` : "The fix"}</span><p>${esc(e.contrast.correct)}</p></div>
    </div>`, true);
  add("steps", "How to decide", e.steps.length && `<ol class="steps">${e.steps
    .map((s) => `<li><span class="q">${esc(s.question)}</span><span class="a">${esc(s.answer)}</span></li>`)
    .join("")}</ol>`);
  add("examples", "More examples", e.examples.length && `<ul class="examples">${e.examples
    .map((x) => `<li><button class="hear" data-hear="${esc(x.sentence)}" aria-label="Hear: ${esc(x.sentence)}">${SPEAKER}</button>
        <div><p class="ex">${focusHTML(x.sentence, x.focus)}</p>${x.note ? `<p class="gloss">${esc(x.note)}</p>` : ""}</div></li>`)
    .join("")}</ul>`, true);
  add("pairs", "Minimal pairs", e.pairs.length && e.pairs
    .map((p) => {
      const [a, b] = pairHTML(p.a, p.b);
      return `<div class="pair"><p class="ex"><span class="lt">A</span><span>${a}</span></p><p class="ex"><span class="lt">B</span><span>${b}</span></p><p class="gloss">${esc(p.difference)}</p></div>`;
    })
    .join(""));
  add("thumb", "Rule of thumb", e.mnemonic && `<p>${esc(e.mnemonic)}</p>`);
  add("native", "Why it slips", e.native && `<p>${esc(e.native)}</p>`);
  add("traps", "Watch out", e.traps.length && `<ul class="traps">${e.traps
    .map((t) => `<li><span class="trap-mark" aria-hidden="true">≠</span><div><p class="ex">${esc(t.trap)}</p><p class="gloss">${esc(t.why)}</p></div></li>`)
    .join("")}</ul>`);
  add("quiz", `Quick check <span class="count" data-score>0 of ${e.quiz.length}</span>`, e.quiz.length && `<ol class="quiz">${e.quiz
    .map((q, k) => `<li data-quiz="${k}"><p class="ex">${gapHTML(q.text)}</p>
        <div class="opts">${q.choices.map((ch) => `<button data-pick="${esc(ch)}">${esc(ch)}</button>`).join("")}</div>
        <p class="gloss why" hidden>${esc(q.why)}</p></li>`)
    .join("")}</ol>`, true);
  add("history", `From your writing <span class="count">×${history.count}</span>`, history.count > 1 && history.slips.length &&
    `<ul class="history">${history.slips.map((s) => `<li><time datetime="${esc(s.date)}">${dayOf(s.date)}</time><p class="ex">${renderDiff(s.before, s.after)}</p></li>`).join("")}</ul>`);

  return `<div class="mods">${mods
    .map((m, i) => `<section class="mod mod-${m.kind}${m.wide ? " wide" : ""}" style="--i:${i}"><h3 class="mod-label">${m.label}</h3>${m.body}</section>`)
    .join("")}</div>`;
}

const explanationSkeleton = () => `<div class="mods" aria-busy="true">
    <div class="mod skel wide"><i></i><i></i><i></i></div>
    <div class="mod skel"><i></i><i></i><i></i><i></i></div>
    <div class="mod skel"><i></i><i></i><i></i></div>
    <div class="mod skel wide"><i></i><i></i><i></i><i></i></div>
  </div>
  <p class="muted wait">Writing a fuller explanation with examples… <span data-elapsed>0 s</span></p>`;

function wireExplanation(root, quiz) {
  root.querySelectorAll("[data-hear]").forEach((b) => b.addEventListener("click", () => say(b.dataset.hear)));
  let right = 0;
  root.querySelectorAll("[data-quiz]").forEach((li) => {
    const q = quiz[Number(li.dataset.quiz)];
    li.querySelectorAll("[data-pick]").forEach((b) =>
      b.addEventListener("click", () => {
        const ok = b.dataset.pick === q.answer;
        const shown = (w) => (w === NONE ? `<span class="zero" title="no word here">‸</span>` : esc(w));
        const slot = li.querySelector("[data-slot]");
        slot.classList.add("filled");
        slot.innerHTML = (ok ? "" : `<del class="slip">${shown(b.dataset.pick)}</del> `) + `<ins class="fix write">${shown(q.answer)}</ins>`;
        li.querySelectorAll("[data-pick]").forEach((o) => {
          o.disabled = true;
          o.classList.add(o.dataset.pick === q.answer ? "right" : o === b ? "wrong" : "dim");
        });
        const why = li.querySelector(".why");
        why.hidden = false;
        why.classList.add(ok ? "ok" : "miss");
        right += ok;
        root.querySelector("[data-score]").textContent = `${right} of ${quiz.length}`;
      })
    );
  });
}

/* ---------- drill player ---------- */

const NONE = "—";
const normGap = (s) => {
  const v = String(s ?? "").trim().toLowerCase();
  return ["", "-", "—", "–", "0", "none", "nothing", "no"].includes(v) ? NONE : v;
};
const normFix = (s) =>
  String(s ?? "")
    .toLowerCase()
    .replace(/’/g, "'")
    .replace(/[^\p{L}\p{N}'\s-]/gu, " ")
    .replace(/\s+/g, " ")
    .trim();

function markFragment(sentence, fragment) {
  const pattern = String(fragment ?? "")
    .trim()
    .split(/\s+/)
    .map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
    .join("\\s+");
  const m = pattern ? sentence.match(new RegExp(pattern, "i")) : null;
  if (!m) return esc(sentence);
  return `${esc(sentence.slice(0, m.index))}<mark>${esc(m[0])}</mark>${esc(sentence.slice(m.index + m[0].length))}`;
}

// where the mistake came from: the whole original sentence, or the slip itself for clozes.
function wroteHTML(c) {
  if (c.source !== "mistake") return "";
  if (c.context) return `<p class="wrote">You wrote: “${markFragment(c.context, c.before)}”</p>`;
  if (c.type === "cloze" && c.before) return `<p class="wrote">You wrote: “<mark>${esc(c.before)}</mark>”</p>`;
  return "";
}

function gapResult(given, answer) {
  const ok = normGap(given) === normGap(answer);
  const zero = `<span class="zero" title="no word here">‸</span>`;
  if (ok) return answer === NONE ? zero : `<ins class="fix write">${esc(answer)}</ins>`;
  const slip = normGap(given) !== NONE ? `<del class="slip">${esc(given)}</del> ` : "";
  return slip + (normGap(answer) === NONE ? zero : `<ins class="fix write">${esc(answer)}</ins>`);
}

function clozeHTML(card, given) {
  const parts = card.text.split("___");
  return parts
    .map((p, i) => {
      if (i === parts.length - 1) return esc(p);
      let gap;
      if (given) gap = gapResult(given[i], card.answers[i]);
      else if (card.choices?.length && parts.length === 2) gap = `<span class="gap" aria-label="blank"></span>`;
      else gap = `<span class="gap"><input data-gap="${i}" aria-label="blank ${i + 1}" autocomplete="off" spellcheck="false"></span>`;
      return esc(p) + gap;
    })
    .join("");
}

async function drill(topic) {
  const [data, lessons, st] = await Promise.all([
    api(`/api/drill?topic=${encodeURIComponent(topic)}&n=10`),
    getLessons(),
    cache.status ? Promise.resolve(cache.status) : getStatus(),
  ]);
  const title = topic === "review" ? "Review" : topicTitle(topic);
  const threshold = st.topics.find((t) => t.id === topic)?.threshold;
  const cards = data.cards;
  if (!cards.length) {
    view.innerHTML = `<section class="leaf"><div class="text">
      <p class="sentence">${topic === "review" ? "Nothing is due right now." : "No exercises in this topic yet."}</p>
      <p class="prose muted">${topic === "review"
        ? "Cards come back 1, 3, 7, 21 and 60 days after you answer them correctly, and the same day when you miss them."
        : "Exercises appear as mistakes in this topic are logged, or when you ask for new ones."}</p>
      <div class="actions">${st.topics.find((t) => t.ready) ? `<a class="button primary" href="#/drill/${esc(st.topics[0].id)}">Practise ${esc(st.topics[0].title)}</a>` : ""}<a class="button" href="#/">Back to today</a></div>
    </div></section>`;
    return;
  }

  const results = [];
  let idx = 0;

  const ticks = () =>
    `<div class="ticks" role="img" aria-label="card ${Math.min(idx + 1, cards.length)} of ${cards.length}">${cards
      .map((_, k) => `<i class="${k < results.length ? (results[k] ? "ok" : "miss") : k === idx ? "now" : ""}"></i>`)
      .join("")}</div>`;
  const summary = lessons[topic]?.summary;

  const askNote = (c) =>
    (c.type === "fix"
      ? `<strong>Rewrite the sentence so it is correct.</strong><p>Edit the text in place, then press Enter.</p>`
      : c.choices?.length && c.text.split("___").length === 2
        ? `<strong>Pick the word that fits.</strong><p>${NONE} means no word belongs there. Keys 1 to ${c.choices.length}.</p>`
        : `<strong>Type the missing words.</strong><p>Leave a blank empty or type ${NONE} when no word belongs there.</p>`) +
    (summary ? `<p>${esc(summary)}</p>` : "");

  const answerNote = (c) => `<strong>${esc(c.rule)}</strong>
      ${c.note ? `<p>${esc(c.note)}</p>` : ""}
      ${c.source === "mistake" && c.date ? `<p>From your writing on ${dayOf(c.date)}.</p>` : c.source === "pack" ? "<p>A new sentence written from your mistakes.</p>" : ""}
      <p><button class="link" data-say>Hear it</button></p>`;

  function frame(inner, note, below = "") {
    view.innerHTML = `<section class="leaf">
      <div class="text">
        <h1>${esc(title)}</h1>
        ${ticks()}
        ${inner}
      </div>
      <aside class="note" id="note">${note}</aside>
      ${below}
    </section>`;
  }

  async function grade(c, ok, given) {
    results.push(ok);
    try {
      await api("/api/answer", { method: "POST", json: { card: c.id, ok, given } });
    } catch (e) {
      toast(`Answer not saved: ${e.message}`);
    }
  }

  function next() {
    idx++;
    if (idx < cards.length) ask();
    else finish();
  }

  function showAnswered(c, inner, ok, selfGrade, given) {
    frame(
      `${inner}
       ${wroteHTML(c)}
       ${selfGrade ? "" : `<p class="verdict ${ok ? "ok" : "miss"}">${ok ? "Right." : "Not quite. The fix is marked in blue."}</p>`}
       <div class="actions" id="after">
         ${selfGrade
          ? `<p class="verdict miss" style="width:100%;margin:0">Close, but not word for word. Was yours right?</p>
               <button class="primary" data-grade="1">Mine was right <kbd>y</kbd></button>
               <button data-grade="0">I missed it <kbd>n</kbd></button>`
            : `<button class="primary" data-next>Next card <kbd>↵</kbd></button>`}
          <button class="link" data-explain>Explain this more</button>
        </div>`,
       answerNote(c),
       `<div class="explain" id="explanation" aria-live="polite"></div>`
     );
    const full = c.after || (c.type === "fix" ? c.answers[0] : "");
    view.querySelector("[data-say]")?.addEventListener("click", () => say(full || c.text.replaceAll("___", c.answers[0])));
    view.querySelector("[data-next]")?.addEventListener("click", next);
    let previous = null;
    view.querySelector("[data-explain]").addEventListener("click", async (e) => {
      const button = e.currentTarget;
      const output = view.querySelector("#explanation");
      button.disabled = true;
      button.textContent = "Explaining…";
      output.innerHTML = explanationSkeleton();
      output.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" });
      const started = Date.now();
      const clock = setInterval(() => {
        const el = output.querySelector("[data-elapsed]");
        if (el) el.textContent = `${Math.round((Date.now() - started) / 1000)} s`;
      }, 1000);
      try {
        const result = await api("/api/explain", { method: "POST", json: { card: c.id, given, previous } });
        previous = result.explanation;
        output.innerHTML = explanationHTML(result.explanation, result.history, c, given);
        wireExplanation(output, result.explanation.quiz);
        button.textContent = "Explain it another way";
      } catch (err) {
        output.innerHTML = `<p class="error">${esc(err.message)}</p>`;
        button.textContent = "Try the explanation again";
      } finally {
        clearInterval(clock);
        button.disabled = false;
      }
    });
    view.querySelectorAll("[data-grade]").forEach((b) =>
      b.addEventListener("click", async () => {
        await grade(c, b.dataset.grade === "1", given);
        next();
      })
    );
    (view.querySelector("[data-next]") || view.querySelector("[data-grade]"))?.focus();
    keyHandler = async (e) => {
      if (selfGrade && (e.key === "y" || e.key === "n")) {
        e.preventDefault();
        await grade(c, e.key === "y", given);
        next();
      } else if (!selfGrade && e.key === "Enter" && !e.target.closest("button, a, input, textarea, select")) {
        e.preventDefault();
        next();
      }
    };
  }

  function ask() {
    const c = cards[idx];
    const single = c.type === "cloze" && c.choices?.length && c.text.split("___").length === 2;
    let inner;
    if (c.type === "fix") {
      inner = `<p class="sentence medium">${esc(c.text)}</p>
        ${wroteHTML(c)}
        <textarea id="answer" rows="3" aria-label="Your corrected sentence" spellcheck="false">${esc(c.text)}</textarea>
        <div class="actions"><button class="primary" data-check>Check <kbd>↵</kbd></button>
          <button class="link" data-skip>Show the answer</button></div>`;
    } else {
      inner = `<p class="sentence">${clozeHTML(c)}</p>
        ${wroteHTML(c)}
        ${single
          ? `<div class="choices">${c.choices.map((ch, k) => `<button data-choice="${esc(ch)}"><kbd>${k + 1}</kbd> ${esc(ch)}</button>`).join("")}</div>`
          : `<div class="actions"><button class="primary" data-check>Check <kbd>↵</kbd></button><button class="link" data-skip>Show the answer</button></div>`}`;
    }
    frame(inner, askNote(c));

    const checkCloze = async (given) => {
      const ok = given.every((g, k) => normGap(g) === normGap(c.answers[k]));
      await grade(c, ok, given.join(" | "));
      showAnswered(c, `<p class="sentence">${clozeHTML(c, given)}</p>`, ok, false, given.join(" | "));
    };
    const checkFix = async (given, skipped) => {
      const ok = !skipped && normFix(given) === normFix(c.answers[0]);
      const inner = `<p class="sentence medium">${renderDiff(given, c.answers[0], true)}</p>`;
      if (ok || skipped) {
        await grade(c, ok, given);
        showAnswered(c, inner, ok, false, given);
      } else {
        showAnswered(c, `${inner}<p class="muted">Expected: ${esc(c.answers[0])}</p>`, false, true, given);
      }
    };

    if (single) {
      view.querySelectorAll("[data-choice]").forEach((b) => b.addEventListener("click", () => checkCloze([b.dataset.choice])));
      keyHandler = (e) => {
        const k = Number(e.key);
        if (k >= 1 && k <= c.choices.length) {
          e.preventDefault();
          checkCloze([c.choices[k - 1]]);
        }
      };
      return;
    }
    const inputs = () => [...view.querySelectorAll("[data-gap]")].map((i) => i.value);
    const submit = () => (c.type === "fix" ? checkFix(view.querySelector("#answer").value, false) : checkCloze(inputs()));
    view.querySelector("[data-check]").addEventListener("click", submit);
    view.querySelector("[data-skip]").addEventListener("click", () =>
      c.type === "fix" ? checkFix(c.text, true) : checkCloze(c.answers.map(() => "?"))
    );
    const first = view.querySelector("#answer, [data-gap]");
    first?.focus();
    if (first?.id === "answer") first.setSelectionRange(first.value.length, first.value.length);
    keyHandler = (e) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        submit();
      }
    };
  }

  async function finish() {
    keyHandler = null;
    const right = results.filter(Boolean).length;
    const missed = [...new Set(cards.filter((_, k) => !results[k]).map((c) => c.rule))];
    try {
      await api("/api/session", { method: "POST", json: { topic, asked: results.length, correct: right } });
    } catch (e) {
      toast(`Session not saved: ${e.message}`);
    }
    cache.status = null;
    view.innerHTML = `<section class="leaf">
      <div class="text">
        <h1>${esc(title)}</h1>
        ${ticks()}
        <p class="sentence">${right} of ${results.length} right.</p>
        ${missed.length ? `<h2 style="margin-top:2rem">Worth another look</h2><ul class="plain">${missed.map((r) => `<li>${esc(r)}</li>`).join("")}</ul>` : ""}
        <div class="actions">
          <button class="primary" data-again>Another round</button>
          ${topic !== "review" ? `<button data-gen>Write 12 new exercises from my mistakes</button>` : ""}
          <a class="button" href="#/">Back to today</a>
        </div>
      </div>
      <aside class="note">${topic === "review"
        ? "<strong>Missed cards come back today.</strong><p>Right ones return after 1, 3, 7, 21 and 60 days.</p>"
        : `<strong>The counter for this topic is back at zero.</strong><p>It fires again after ${threshold} new mistakes. Missed cards return in the review.</p>`}</aside>
    </section>`;
    view.querySelector("[data-again]").addEventListener("click", () => route());
    const gen = view.querySelector("[data-gen]");
    gen?.addEventListener("click", async () => {
      gen.disabled = true;
      gen.textContent = "Writing exercises from your mistakes, about 30 seconds…";
      try {
        const r = await api(`/api/packs/${encodeURIComponent(topic)}`, { method: "POST" });
        toast(`${r.added} new exercises added to ${title}.`);
        gen.textContent = `${r.added} exercises added`;
      } catch (e) {
        gen.disabled = false;
        gen.textContent = "Write 12 new exercises from my mistakes";
        toast(`Could not write exercises: ${e.message}`);
      }
    });
    keyHandler = (e) => {
      if (e.key === "Enter") route();
    };
  }

  ask();
}

/* ---------- speaking ---------- */

function pickMime() {
  for (const [mime, ext] of [["audio/webm;codecs=opus", ".webm"], ["audio/webm", ".webm"], ["audio/mp4", ".mp4"], ["audio/ogg;codecs=opus", ".ogg"]])
    if (window.MediaRecorder?.isTypeSupported(mime)) return { mime, ext };
  return { mime: "", ext: ".webm" };
}

function costBadge(rec) {
  const parts = [];
  if (rec.whisper_cost > 0) parts.push(`transcription $${rec.whisper_cost.toFixed(4)}`);
  if (rec.review_cost > 0) parts.push(`review $${rec.review_cost.toFixed(4)}`);
  return parts.length ? `<span class="cost">${parts.join(" · ")}</span>` : "";
}

function reviewHTML(rec) {
  const r = rec.review;
  if (!r) return "";
  const errs = (r.errors || [])
    .map((e) => `<li><span class="example">${renderDiff(e.before, e.after)}</span><br><span class="muted">${esc(e.rule)}</span></li>`)
    .join("");
  return `<h3 style="margin-top:2rem">Corrected</h3>
    <p class="sentence small">${renderDiff(rec.transcript, r.corrected, true)}</p>
    <div class="actions"><button data-hear="${esc(r.corrected)}">Hear the corrected version</button></div>
    ${errs ? `<h3 style="margin-top:2rem">What to fix</h3><ul class="plain">${errs}</ul>` : `<p class="verdict ok">No grammar mistakes found.</p>`}
    ${r.tips?.length ? `<h3 style="margin-top:2rem">Tips</h3><ul class="plain">${r.tips.map((t) => `<li>${esc(t)}</li>`).join("")}</ul>` : ""}
    ${r.fluency ? `<p class="prose muted">${esc(r.fluency)}</p>` : ""}
    ${costBadge(rec)}`;
}

function starBar(score) {
  return `<span class="stars" aria-label="${score} out of 5">${[1, 2, 3, 4, 5].map((i) => `<i class="${i <= score ? "on" : ""}"></i>`).join("")}</span>`;
}

function interviewReviewHTML(rec) {
  const r = rec.interview_review;
  if (!r) return "";
  const dims = ["situation", "task", "action", "result"];
  const rows = dims
    .map((d) => {
      const s = r.star[d];
      return `<tr><td>${esc(d[0].toUpperCase() + d.slice(1))}</td><td>${starBar(s.score)}</td><td>${esc(s.feedback)}</td></tr>`;
    })
    .join("");
  return `<h3 style="margin-top:2rem">STAR review</h3>
    <table class="star-table"><tbody>${rows}</tbody></table>
    <p><strong>Overall:</strong> ${starBar(r.overall_score)}</p>
    ${r.rewrite ? `<h3 style="margin-top:2rem">Tighter version</h3><p class="sentence small">${esc(r.rewrite)}</p>` : ""}
    ${r.story_match ? `<p class="prose muted">Best story match: ${esc(r.story_match)}</p>` : ""}
    ${r.strengths?.length ? `<h3 style="margin-top:2rem">Strengths</h3><ul class="plain">${r.strengths.map((x) => `<li>${esc(x)}</li>`).join("")}</ul>` : ""}
    ${r.gaps?.length ? `<h3 style="margin-top:2rem">Gaps</h3><ul class="plain">${r.gaps.map((x) => `<li>${esc(x)}</li>`).join("")}</ul>` : ""}
    ${r.follow_ups?.length ? `<h3 style="margin-top:2rem">Likely follow-ups</h3><ul class="plain">${r.follow_ups.map((x) => `<li>${esc(x)}</li>`).join("")}</ul>` : ""}
    ${costBadge(rec)}`;
}

function wireHear(root) {
  root.querySelectorAll("[data-hear]").forEach((b) => b.addEventListener("click", () => say(b.dataset.hear)));
}

async function speakView(modeHint = "grammar", promptHint = "") {
  const qReq = promptHint ? Promise.resolve({ prompt: promptHint, focus: "" }) : api("/api/speak/question");
  const [st, q, history] = await Promise.all([getStatus(), qReq, api("/api/speak")]);
  const tools = st.speaking.tools;
  const whisperOk = st.whisper?.openrouter_ok || st.whisper?.local_ready;
  let recorder = null, chunks = [], started = 0, tick = null, stream = null;
  let reviewMode = modeHint;

  const backendNote = `<p class="muted backend">Transcription: ${esc(st.whisper?.backend || "local")} · ${esc(st.whisper?.model || "whisper.cpp")}${st.speaking.spend ? ` · spend $${st.speaking.spend.toFixed(4)}` : ""}</p>`;

  const hist = history
    .map(
      (r) => `<li><details><summary><span class="muted">${when(r.ts)}</span> ${esc(r.prompt)}
        <span class="muted">— ${r.words} words, ${r.review ? `${(r.review.errors || []).length} to fix` : r.interview_review ? "STAR review" : "not checked yet"}</span></summary>
        <p class="sentence small">${esc(r.transcript)}</p>${r.interview_review ? interviewReviewHTML(r) : reviewHTML(r)}</details></li>`
    )
    .join("");

  view.innerHTML = `<section class="leaf">
      <div class="text">
        <p class="sentence medium">${esc(q.prompt)}</p>
        ${whisperOk ? "" : `<div class="callout">Recording works once these are in place: ${esc(tools.missing || "OpenRouter key or local whisper-cpp")}.<br>Run <code>eng doctor</code> in a terminal, then reload.</div>`}
        ${st.llm.ok ? "" : `<div class="callout">The grammar check needs OpenRouter or the claude CLI; <code>eng doctor</code> shows which one is missing.</div>`}
        ${backendNote}
        <div class="actions">
          <button class="record primary" id="rec" ${whisperOk ? "" : "disabled"}><span class="dot"></span><span id="rec-label">Record</span> <kbd>space</kbd></button>
          <span class="timer" id="timer"></span>
          <select id="mode" aria-label="Review mode">
            <option value="grammar" ${reviewMode === "grammar" ? "selected" : ""}>Check grammar</option>
            <option value="star" ${reviewMode === "star" ? "selected" : ""}>Check as interview answer (STAR)</option>
          </select>
          <button class="link" id="another">Another question</button>
        </div>
        <div id="out"></div>
      </div>
      <aside class="note"><strong>Talk for one to two minutes.</strong>
        ${q.focus ? `<p>Try to get this one right: ${esc(q.focus)}.</p>` : ""}
        <p>Whisper writes down what you say; the check ignores fillers and punctuation.</p></aside>
    </section>
    <section class="section"><h2>Earlier answers</h2>${hist ? `<ul class="plain">${hist}</ul>` : `<p class="muted">Your recorded answers will be listed here.</p>`}</section>`;
  wireHear(view);

  const out = view.querySelector("#out");
  const recBtn = view.querySelector("#rec");
  const label = view.querySelector("#rec-label");
  const timer = view.querySelector("#timer");
  const modeSelect = view.querySelector("#mode");
  modeSelect.addEventListener("change", () => { reviewMode = modeSelect.value; });
  view.querySelector("#another").addEventListener("click", () => route());

  async function start() {
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch (e) {
      out.innerHTML = `<p class="error">The microphone is blocked: ${esc(e.message)}. Allow it for this page in the browser settings.</p>`;
      return;
    }
    const { mime } = pickMime();
    recorder = new MediaRecorder(stream, mime ? { mimeType: mime } : undefined);
    chunks = [];
    recorder.ondataavailable = (e) => e.data.size && chunks.push(e.data);
    recorder.onstop = upload;
    recorder.start();
    started = Date.now();
    recBtn.classList.add("live");
    label.textContent = "Stop";
    tick = setInterval(() => {
      const s = Math.round((Date.now() - started) / 1000);
      timer.textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
    }, 250);
  }

  function stop() {
    if (!recorder || recorder.state !== "recording") return;
    recorder.stop();
    stream.getTracks().forEach((t) => t.stop());
    clearInterval(tick);
    recBtn.classList.remove("live");
    label.textContent = "Record again";
  }

  async function upload() {
    const { ext } = pickMime();
    const blob = new Blob(chunks, { type: recorder.mimeType || "audio/webm" });
    const form = new FormData();
    form.append("audio", blob, "answer" + ext);
    form.append("prompt", q.prompt);
    form.append("focus", q.focus || "");
    out.innerHTML = `<p class="muted">Transcribing ${timer.textContent} of speech…</p>`;
    recBtn.disabled = true;
    try {
      const rec = await api("/api/speak", { method: "POST", body: form });
      const canCheck = rec.transcript && st.llm.ok;
      out.innerHTML = `<h3 style="margin-top:2rem">What Whisper heard</h3>
        <p class="sentence small">${esc(rec.transcript) || "<span class='muted'>Nothing was recognised.</span>"}</p>
        <div class="actions"><button class="primary" id="check" ${canCheck ? "" : "disabled"}>${reviewMode === "star" ? "Check as STAR answer" : "Check my grammar"}</button>
        <span class="muted">${rec.words} words · ${esc(rec.whisper_backend)}</span></div><div id="review"></div>`;
      const check = out.querySelector("#check");
      check.addEventListener("click", async () => {
        check.disabled = true;
        check.textContent = "Checking, about 15 seconds…";
        try {
          const done = await api(`/api/speak/${encodeURIComponent(rec.id)}/review?mode=${reviewMode}`, { method: "POST" });
          out.querySelector("#review").innerHTML = reviewMode === "star" ? interviewReviewHTML(done) : reviewHTML(done);
          wireHear(out);
          check.textContent = "Checked";
          cache.mistakes = null;
        } catch (e) {
          check.disabled = false;
          check.textContent = reviewMode === "star" ? "Check as STAR answer" : "Check my grammar";
          toast(`Check failed: ${e.message}`);
        }
      });
      check.focus();
    } catch (e) {
      out.innerHTML = `<p class="error">Transcription failed: ${esc(e.message)}</p>`;
    } finally {
      recBtn.disabled = false;
    }
  }

  recBtn.addEventListener("click", () => (recorder?.state === "recording" ? stop() : start()));
  keyHandler = (e) => {
    if (e.code === "Space" && !["TEXTAREA", "INPUT", "BUTTON", "SUMMARY", "SELECT"].includes(document.activeElement?.tagName) && whisperOk) {
      e.preventDefault();
      recorder?.state === "recording" ? stop() : start();
    }
  };
  cleanup = () => {
    if (recorder?.state === "recording") {
      recorder.onstop = null;
      recorder.stop();
    }
    stream?.getTracks().forEach((t) => t.stop());
    clearInterval(tick);
  };
}

/* ---------- work english ---------- */

async function workView(topic) {
  const [data, st] = await Promise.all([getWorkEnglish(), cache.status ? Promise.resolve(cache.status) : getStatus()]);
  const cats = data.categories || [];
  let active = cats.find((c) => c.id === topic) || cats[0];

  const nav = `<nav class="lesson-nav" aria-label="Work English topics">${cats
    .map((c) => `<a href="#/work/${esc(c.id)}" ${c.id === active?.id ? 'aria-current="page"' : ""}>${esc(c.title)}</a>`)
    .join("")}</nav>`;

  const categoryHTML = (c) => `
    <section class="leaf">
      <div class="text">
        <h1>${esc(c.title)}</h1>
        ${c.phrases?.length ? `<h2>Useful phrases</h2><ul class="plain phrases">${c.phrases.map((p) => `<li><p>${esc(p.text)}</p>${p.notes ? `<p class="muted">${esc(p.notes)}</p>` : ""}</p></li>`).join("")}</ul>` : ""}
        ${c.prompts?.length ? `<h2>Practice prompts</h2><ul class="plain prompts">${c.prompts.map((p) => `<li><p>${esc(p)}</p><a class="button" href="#/speak?mode=${c.id === "interview" ? "star" : "grammar"}&prompt=${encodeURIComponent(p)}">Record an answer</a></li>`).join("")}</ul>` : ""}
      </div>
      <aside class="note"><strong>${esc(c.title)}</strong><p>Pick a prompt, record your answer, and choose grammar or STAR feedback.</p></aside>
    </section>`;

  view.innerHTML = `${nav}${active ? categoryHTML(active) : `<p class="muted">No Work English content yet.</p>`}`;
}

/* ---------- check a pasted text ---------- */

const KIND_LABEL = { grammar: "grammar", punctuation: "punct", lexical: "word choice", spelling: "spelling", style: "style" };

async function checkView() {
  const st = cache.status ? cache.status : await getStatus();
  view.innerHTML = `<section class="leaf">
      <div class="text">
        <h1>Check a text before you send it</h1>
        <p class="prose muted">Paste an email, an MR description or a message. The LLM marks what to fix; what you confirm joins the mistake pile and comes back as drills.</p>
        <textarea id="paste" rows="8" placeholder="Paste your English here…" spellcheck="false"></textarea>
        <div class="actions">
          <button class="primary" id="run" disabled>Check <kbd>↵</kbd></button>
          <button class="link" id="clear" hidden>Clear</button>
          <span class="muted" id="meta"></span>
        </div>
        <div id="out"></div>
      </div>
      <aside class="note"><strong>Nothing is logged until you say so.</strong>
        <p>Style notes are only advice — they never enter the database.</p>
        ${st.llm.ok ? "" : `<p class="callout">No LLM backend: run <code>eng doctor</code>.</p>`}</aside>
    </section>`;
  const ta = view.querySelector("#paste");
  const run = view.querySelector("#run");
  const clearBtn = view.querySelector("#clear");
  const out = view.querySelector("#out");
  const meta = view.querySelector("#meta");
  let draft = null;
  const explainers = new Map();

  ta.addEventListener("input", () => {
    const nWords = words(ta.value).length;
    run.disabled = !ta.value.trim();
    meta.textContent = ta.value.trim() ? `${nWords} words` : "";
  });
  clearBtn.addEventListener("click", () => {
    ta.value = "";
    run.disabled = true;
    clearBtn.hidden = true;
    out.innerHTML = "";
    meta.textContent = "";
    draft = null;
    explainers.clear();
    ta.focus();
  });

  const mistakeRow = (e, count, k) => `<li>
      <p class="sentence small">${renderDiff(e.before, e.after)}</p>
      <div class="note"><strong>${esc(e.rule)}</strong>
        <p><span class="tag kind">${esc(KIND_LABEL[e.category] || e.category)}</span>${count ? ` <span class="muted">×${count}</span>` : ""}${e.note ? ` — ${esc(e.note)}` : ""}</p>
        <p><button class="link" data-explain-mistake="${k}">Explain this more</button></p>
      </div>
      <div class="explain" data-explanation="${k}" aria-live="polite"></div>
    </li>`;

  function wireExplainers() {
    view.querySelectorAll("[data-explain-mistake]").forEach((b) =>
      b.addEventListener("click", async () => {
        const k = Number(b.dataset.explainMistake);
        const e = draft.errors[k];
        const out2 = view.querySelector(`[data-explanation="${k}"]`);
        const prev = explainers.get(k);
        b.disabled = true;
        b.textContent = "Explaining…";
        out2.innerHTML = explanationSkeleton();
        out2.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" });
        try {
          const res = await api("/api/check/explain", {
            method: "POST",
            json: { category: e.category, rule: e.rule, before: e.before, after: e.after, note: e.note, previous: prev || null },
          });
          explainers.set(k, res.explanation);
          const fake = { type: "fix", answers: [e.after], rule: e.rule, note: e.note };
          out2.innerHTML = explanationHTML(res.explanation, res.history, fake, e.before);
          wireExplanation(out2, res.explanation.quiz);
          wireHear(out2);
          b.textContent = "Explain it another way";
        } catch (err) {
          out2.innerHTML = `<p class="error">${esc(err.message)}</p>`;
          b.textContent = "Try the explanation again";
        } finally {
          b.disabled = false;
        }
      })
    );
  }

  async function check() {
    const text = ta.value.trim();
    if (!text) return;
    run.disabled = true;
    run.textContent = "Checking…";
    out.innerHTML = `<p class="muted wait">Reading your text… <span data-elapsed>0 s</span></p>`;
    clearBtn.hidden = false;
    const started = Date.now();
    const clock = setInterval(() => {
      const el = out.querySelector("[data-elapsed]");
      if (el) el.textContent = `${Math.round((Date.now() - started) / 1000)} s`;
    }, 1000);
    try {
      draft = await api("/api/check", { method: "POST", json: { text } });
      renderResult();
    } catch (e) {
      out.innerHTML = `<p class="error">${esc(e.message)}</p>`;
    } finally {
      clearInterval(clock);
      run.disabled = false;
      run.textContent = "Check again";
    }
  }

  function renderResult() {
    if (!draft) return;
    draft.errors = draft.errors || [];
    const fixable = draft.errors.map((e, k) => ({ e, k })).filter(({ e }) => e.category !== "style");
    const style = draft.errors.map((e, k) => ({ e, k })).filter(({ e }) => e.category === "style");
    const counted = draft.counted || {};
    out.innerHTML = `
      <h3 style="margin-top:2rem">Corrected</h3>
      <p class="sentence small">${draft.corrected === draft.original ? esc(draft.corrected) : renderDiff(draft.original, draft.corrected, true)}</p>
      <div class="actions">
        <button data-hear="${esc(draft.corrected)}">Hear it</button>
        ${draft.cost ? `<span class="cost">$${draft.cost.toFixed(4)}</span>` : ""}
      </div>
      ${fixable.length
        ? `<h3 style="margin-top:2rem">What to fix <span class="muted">(${fixable.length})</span></h3><ul class="plain">${fixable.map(({ e, k }) => mistakeRow(e, counted[e.rule], k)).join("")}</ul>
           <div class="actions"><button class="primary" id="log">Add ${fixable.length} to the mistake pile</button></div>`
        : `<p class="verdict ok">No grammar mistakes found.</p>`}
      ${style.length
        ? `<h3 style="margin-top:2rem">Style notes <span class="muted">(advice, not logged)</span></h3><ul class="plain">${style.map(({ e, k }) => mistakeRow(e, 0, k)).join("")}</ul>` : ""}`;
    wireHear(out);
    wireExplainers();
    const logBtn = out.querySelector("#log");
    logBtn?.addEventListener("click", async () => {
      logBtn.disabled = true;
      logBtn.textContent = "Writing…";
      try {
      const res = await api(`/api/check/${encodeURIComponent(draft.id)}/log`, { method: "POST" });
      toast(`${res.logged} mistakes logged. The mistakes view and drills now know about them.`);
      logBtn.textContent = `Logged ${res.logged}`;
        cache.mistakes = null;
        cache.status = null;
      } catch (e) {
        logBtn.disabled = false;
        logBtn.textContent = "Add to the mistake pile";
        toast(`Not logged: ${e.message}`);
      }
    });
  }

  run.addEventListener("click", check);
  keyHandler = (e) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      check();
    }
  };
}

/* ---------- growth card ---------- */

const pct = (v) => `${Math.round(v * 100)}%`;

function deltaHTML(now, prev) {
  if (!prev || !now) return "";
  const drop = Math.round(((prev - now) / prev) * 100);
  if (drop > 0) return `<span class="delta down">−${drop}%</span>`;
  if (drop < 0) return `<span class="delta up">+${-drop}%</span>`;
  return `<span class="delta">±0%</span>`;
}

async function growthCard(anchor) {
  anchor.disabled = true;
  anchor.textContent = "Collecting your numbers…";
  let section;
  try {
    const st = await api("/api/growth");
    section = document.createElement("section");
    section.className = "section leaf growth";
    section.innerHTML = `
      <div class="text">
        <h2>Are you actually getting better?</h2>
        <div class="figures" aria-busy="true"><div class="skel"></div><div class="skel"></div><div class="skel"></div><div class="skel"></div></div>
      </div>`;
    anchor.closest("section").after(section);
    const cheerPromise = st.llm_ok === false ? Promise.resolve(null) : api("/api/growth/cheer", { method: "POST" }).catch(() => null);    const months = (st.months || []).slice(-6);
    const maxRate = Math.max(1, ...months.map((m) => m.rate));
    const bars = months
      .map((m, i) => {
        const h = Math.max(2, (m.rate / maxRate) * 70);
        return `<rect class="${m.month === months[months.length - 1].month ? "bar-now" : "bar-rate"}" x="${i * 40 + 8}" y="${84 - h}" width="24" height="${h}" rx="3"><title>${m.month}: ${m.rate.toFixed(1)} per 100 words over ${m.words} words</title></rect>
          <text x="${i * 40 + 20}" y="100" text-anchor="middle">${m.month.slice(5)}</text>`;
      })
      .join("");
    const w = st.week_rate, pw = st.prev_week_rate;
    const figures = `
      <div><b>${st.streak}</b>${st.streak === 1 ? "day streak" : "day streak"}${st.best_streak ? ` <span class="muted">best ${st.best_streak}</span>` : ""}</div>
      <div><b>${w ? w.toFixed(1) : "—"}</b>errors/100 words this week ${deltaHTML(w, pw)}</div>
      <div><b>${pct(st.drill_accuracy)}</b>drill accuracy, last 20</div>
      <div><b>${st.ripe_topics}/${st.total_topics}</b>topics ripe right now</div>`;
    const facts = [];
    if (st.peak_day_rate) facts.push(`Your worst day ever peaked at <b>${st.peak_day_rate.toFixed(1)}</b> errors/100 words.`);
    if (st.best_month_rate && st.current_rate && st.current_rate > st.best_month_rate) facts.push(`Best month so far: <b>${st.best_month_rate.toFixed(1)}</b>/100 — this month you are at <b>${st.current_rate.toFixed(1)}</b>.`);
    if (st.top_rule) facts.push(`All-time champion: “${esc(st.top_rule)}” — <b>${times(st.top_rule_count)}</b>.`);
    if (st.new_rules?.length) facts.push(`Fresh quarries this month: ${st.new_rules.map((r) => `“${esc(r.rule)}”`).join(", ")}.`);
    let cardHTML = "";
    try {
      const card = await cheerPromise;
      if (card) {
        cardHTML = `<p class="sentence headline">${esc(card.headline)}</p>
          <p class="prose">${esc(card.cheer)}</p>
          ${card.nudge ? `<p class="prose muted">${esc(card.nudge)}</p>` : ""}
          <p class="cost">${esc(card.backend || "")}</p>`;
      }
    } catch {}
    if (!cardHTML) {
      cardHTML = w && pw && w < pw
        ? `<p class="sentence headline">Rate is down ${Math.round(((pw - w) / pw) * 100)}% week over week.</p>`
        : `<p class="sentence headline">Every logged mistake is one you will not make twice.</p>`;
    }
    section.innerHTML = `
      <div class="text">
        <h2>Are you actually getting better?</h2>
        ${cardHTML}
        <div class="figures">${figures}</div>
        ${months.length > 1 ? `<svg class="chart" viewBox="0 0 ${months.length * 40 + 16} 108" role="img" aria-label="Monthly mistake rate">${bars}</svg>` : ""}
        <ul class="plain facts">${facts.map((f) => `<li>${f}</li>`).join("")}</ul>
        <div class="actions"><a class="button primary" href="#/review">Review due cards</a></div>
      </div>
      <aside class="note"><strong>Lower is better.</strong><p>The rate counts only checked prompt words; texts you paste in Check are not in it.</p></aside>`;
  } catch (e) {
    toast(`Growth card failed: ${e.message}`);
  } finally {
    anchor.disabled = false;
    anchor.textContent = "Cheer me up";
    if (section) section.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" });
  }
}

/* ---------- mistakes ---------- */

async function mistakesView() {
  const [list, st] = await Promise.all([getMistakes(), cache.status ? Promise.resolve(cache.status) : getStatus()]);
  let topic = "";
  let query = "";
  view.innerHTML = `<h1>Everything you have written wrong, corrected</h1>
    <div class="filters" role="group" aria-label="Filter by topic">
      <button aria-pressed="true" data-topic="">All ${list.length}</button>
      ${st.topics.map((t) => `<button aria-pressed="false" data-topic="${esc(t.id)}">${esc(t.title)} ${t.total}</button>`).join("")}
    </div>
    <input type="search" id="q" placeholder="Search your sentences and rules" aria-label="Search mistakes">
    <ul class="rows" id="list" style="margin-top:1.5rem"></ul>`;
  const ul = view.querySelector("#list");
  function draw() {
    const q = query.toLowerCase();
    const rows = list.filter(
      (m) => (!topic || m.topics.includes(topic)) && (!q || `${m.before} ${m.after} ${m.rule}`.toLowerCase().includes(q))
    );
    ul.innerHTML = rows.length
      ? rows
          .slice(0, 300)
          .map(
            (m) => `<li><p class="sentence small">${renderDiff(m.before, m.after)}</p>
            <div class="note"><strong>${esc(m.rule)}</strong><p>${esc(m.category)}, ${dayOf(m.date)}</p>${m.note ? `<p>${esc(m.note)}</p>` : ""}</div></li>`
          )
          .join("")
      : `<li><p class="muted">Nothing matches.</p></li>`;
  }
  view.querySelectorAll("[data-topic]").forEach((b) =>
    b.addEventListener("click", () => {
      topic = b.dataset.topic;
      view.querySelectorAll("[data-topic]").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
      draw();
    })
  );
  view.querySelector("#q").addEventListener("input", (e) => {
    query = e.target.value;
    draw();
  });
  draw();
}

/* ---------- lessons ---------- */

async function lessonsView(topic) {
  const [lessons, list, st] = await Promise.all([getLessons(), getMistakes(), cache.status ? Promise.resolve(cache.status) : getStatus()]);
  topic = topic || st.topics[0]?.id;
  const lesson = lessons[topic];
  const mine = list.filter((m) => m.topics.includes(topic)).slice(0, 8);
  view.innerHTML = `<nav class="lesson-nav" aria-label="Topics">${st.topics
    .map((t) => `<a href="#/lessons/${esc(t.id)}" ${t.id === topic ? 'aria-current="page"' : ""}>${esc(t.title)}</a>`)
    .join("")}</nav>
    ${lesson
      ? `<section class="leaf"><div class="text"><p class="sentence medium">${esc(lesson.summary)}</p>
          <div class="actions"><a class="button primary" href="#/drill/${esc(topic)}">Practise ${esc(topicTitle(topic))}</a></div></div></section>
        ${lesson.rules
          .map(
            (r) => `<section class="section leaf"><div class="text"><h2>${esc(r.title)}</h2><p class="prose">${esc(r.body)}</p>
            ${(r.examples || []).map((x) => `<p class="example">${renderDiff(x.bad, x.good)}</p>`).join("")}</div></section>`
          )
          .join("")}`
      : `<p class="muted">No lesson written for this topic yet.</p>`}
    ${mine.length
      ? `<section class="section"><h2>From your own writing</h2><ul class="rows">${mine
          .map((m) => `<li><p class="sentence small">${renderDiff(m.before, m.after)}</p><div class="note"><strong>${esc(m.rule)}</strong><p>${dayOf(m.date)}</p></div></li>`)
          .join("")}</ul></section>`
      : ""}`;
}

/* ---------- router ---------- */

const routes = [
  [/^#?\/?$/, () => today(), "today"],
  [/^#\/drill\/([\w-]+)$/, (m) => drill(m[1]), null],
  [/^#\/review$/, () => drill("review"), "review"],
  [/^#\/speak(?:\?.*)?$/, () => speakFromHash(), "speak"],
  [/^#\/work(?:\/([\w-]+))?$/, (m) => workView(m[1]), "work"],
  [/^#\/check$/, () => checkView(), "check"],
  [/^#\/mistakes$/, () => mistakesView(), "mistakes"],
  [/^#\/lessons(?:\/([\w-]+))?$/, (m) => lessonsView(m[1]), "lessons"],
];

function speakFromHash() {
  const params = new URLSearchParams(location.hash.split("?")[1] || "");
  speakView(params.get("mode") || "grammar", params.get("prompt") || "");
}

async function route() {
  keyHandler = null;
  if (cleanup) {
    cleanup();
    cleanup = null;
  }
  if ("speechSynthesis" in window) speechSynthesis.cancel();
  const hash = location.hash || "#/";
  const hit = routes.find(([re]) => re.test(hash)) || routes[0];
  document.querySelectorAll("[data-nav]").forEach((a) => {
    if (a.dataset.nav === hit[2]) a.setAttribute("aria-current", "page");
    else a.removeAttribute("aria-current");
  });
  view.innerHTML = `<p class="muted">Loading…</p>`;
  try {
    await hit[1](hash.match(hit[0]));
  } catch (e) {
    view.innerHTML = `<section class="leaf"><div class="text"><p class="sentence medium">Something broke while loading this page.</p>
      <p class="error">${esc(e.message)}</p><p class="muted">Run <code>eng doctor</code> in a terminal to see what is missing.</p></div></section>`;
  }
  view.focus({ preventScroll: true });
  window.scrollTo(0, 0);
}

window.addEventListener("hashchange", route);
route();
