"use strict";

const view = document.getElementById("view");
const toastEl = document.getElementById("toast");
let keyHandler = null;
let cleanup = null;
const cache = { mistakes: null, lessons: null, status: null };

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
async function getStatus() {
  cache.status = await api("/api/status");
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

  const heroHTML = hero
    ? `<section class="leaf">
        <div class="text">
          <p class="sentence hero">${renderDiff(hero.entry.before, hero.entry.after, true)}</p>
          <div class="actions">
            <a class="button primary" href="#/drill/${esc(heroTopic)}">Practise ${esc(topicTitle(heroTopic))}</a>
            ${st.due ? `<a class="button" href="#/review">Review ${st.due} due</a>` : ""}
          </div>
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

  keyHandler = (e) => {
    if (e.key === "Enter" && heroTopic && document.activeElement === view) location.hash = `#/drill/${heroTopic}`;
  };
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

  function frame(inner, note) {
    view.innerHTML = `<section class="leaf">
      <div class="text">
        <h1>${esc(title)}</h1>
        ${ticks()}
        ${inner}
      </div>
      <aside class="note" id="note">${note}</aside>
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
       `${answerNote(c)}<div id="explanation" aria-live="polite"></div>`
     );
    const full = c.after || (c.type === "fix" ? c.answers[0] : "");
    view.querySelector("[data-say]")?.addEventListener("click", () => say(full || c.text.replaceAll("___", c.answers[0])));
    view.querySelector("[data-next]")?.addEventListener("click", next);
    view.querySelector("[data-explain]").addEventListener("click", async (e) => {
      const button = e.currentTarget;
      const output = view.querySelector("#explanation");
      button.disabled = true;
      button.textContent = "Explaining…";
      output.innerHTML = `<p class="muted">Preparing an explanation with the full card context…</p>`;
      try {
        const result = await api("/api/explain", { method: "POST", json: { card: c.id, given } });
        output.innerHTML = `<p class="prose">${esc(result.explanation)}</p>`;
        button.disabled = false;
        button.textContent = "Explain this another way";
      } catch (err) {
        output.innerHTML = `<p class="error">${esc(err.message)}</p>`;
        button.disabled = false;
        button.textContent = "Try the explanation again";
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
    ${r.fluency ? `<p class="prose muted">${esc(r.fluency)}</p>` : ""}`;
}

function wireHear(root) {
  root.querySelectorAll("[data-hear]").forEach((b) => b.addEventListener("click", () => say(b.dataset.hear)));
}

async function speakView() {
  const [st, q, history] = await Promise.all([getStatus(), api("/api/speak/question"), api("/api/speak")]);
  const tools = st.speaking.tools;
  let recorder = null, chunks = [], started = 0, tick = null, stream = null;

  const hist = history
    .map(
      (r) => `<li><details><summary><span class="muted">${when(r.ts)}</span> ${esc(r.prompt)}
        <span class="muted">— ${r.words} words, ${r.review ? `${(r.review.errors || []).length} to fix` : "not checked yet"}</span></summary>
        <p class="sentence small">${esc(r.transcript)}</p>${reviewHTML(r)}</details></li>`
    )
    .join("");

  view.innerHTML = `<section class="leaf">
      <div class="text">
        <p class="sentence medium">${esc(q.prompt)}</p>
        ${tools.ready ? "" : `<div class="callout">Recording works once these are in place: ${esc(tools.missing)}.<br>Run <code>eng whisper-setup</code> in a terminal, then reload.</div>`}
        ${st.llm.ok ? "" : `<div class="callout">The grammar check needs OpenRouter or the claude CLI; <code>eng doctor</code> shows which one is missing.</div>`}
        <div class="actions">
          <button class="record primary" id="rec" ${tools.ready ? "" : "disabled"}><span class="dot"></span><span id="rec-label">Record</span> <kbd>space</kbd></button>
          <span class="timer" id="timer"></span>
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
      out.innerHTML = `<h3 style="margin-top:2rem">What Whisper heard</h3>
        <p class="sentence small">${esc(rec.transcript) || "<span class='muted'>Nothing was recognised.</span>"}</p>
        <div class="actions"><button class="primary" id="check" ${rec.transcript && st.llm.ok ? "" : "disabled"}>Check my grammar</button>
        <span class="muted">${rec.words} words</span></div><div id="review"></div>`;
      const check = out.querySelector("#check");
      check.addEventListener("click", async () => {
        check.disabled = true;
        check.textContent = "Checking, about 15 seconds…";
        try {
          const done = await api(`/api/speak/${encodeURIComponent(rec.id)}/review`, { method: "POST" });
          out.querySelector("#review").innerHTML = reviewHTML(done);
          wireHear(out);
          check.textContent = "Checked";
          cache.mistakes = null;
        } catch (e) {
          check.disabled = false;
          check.textContent = "Check my grammar";
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
    if (e.code === "Space" && !["TEXTAREA", "INPUT", "BUTTON", "SUMMARY"].includes(document.activeElement?.tagName) && tools.ready) {
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
  [/^#\/speak$/, () => speakView(), "speak"],
  [/^#\/mistakes$/, () => mistakesView(), "mistakes"],
  [/^#\/lessons(?:\/([\w-]+))?$/, (m) => lessonsView(m[1]), "lessons"],
];

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
