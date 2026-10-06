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

/* ---------- signage: pictograms, the sign, signposts ---------- */

const PICTO = {
  arrow: `<path d="M4.5 12h15M13.5 6l6 6-6 6"/>`,
  cards: `<rect x="3.5" y="7" width="13" height="13" rx="1.5"/><path d="M8 3.5h11a1.5 1.5 0 0 1 1.5 1.5v11"/>`,
  doc: `<path d="M6 3h8.5L19 7.5V21H6z"/><path d="M14 3v5h5M9.5 12.5h6M9.5 16.5h6"/>`,
  letter: `<path d="M5.5 19.5 12 4.5l6.5 15M8 14h8"/>`,
  turn: `<path d="M6 4v7a4 4 0 0 0 4 4h9M15 11l4 4-4 4"/>`,
  clock: `<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>`,
  list: `<path d="M9 6.5h11M9 12h11M9 17.5h11M4.5 6.5h.01M4.5 12h.01M4.5 17.5h.01"/>`,
  comma: `<circle cx="12" cy="9" r="2.2"/><path d="M14.2 9.5c0 3.5-1.8 6.5-5 8"/>`,
  chat: `<path d="M4 5h16v11H10l-5 4v-4H4z"/>`,
  pen: `<path d="m4 20 1-4.5L16 4.5l3.5 3.5L8.5 19z"/><path d="m13.5 7 3.5 3.5"/>`,
  mic: `<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21"/>`,
  person: `<circle cx="12" cy="7.5" r="3.5"/><path d="M5 20.5a7 7 0 0 1 14 0"/>`,
  alert: `<path d="M12 3.5 21 19.5H3z"/><path d="M12 10v4M12 16.8h.01"/>`,
  check: `<path d="m5 12.5 4.5 4.5L19 7"/>`,
  help: `<circle cx="12" cy="12" r="8.5"/><path d="M9.6 9.6a2.5 2.5 0 1 1 3.4 2.3c-.6.3-1 .8-1 1.5v.6M12 16.8h.01"/>`,
  book: `<path d="M12 6.5C10 5 7 4.6 3.5 5v13.5c3.5-.4 6.5 0 8.5 1.5 2-1.5 5-1.9 8.5-1.5V5c-3.5-.4-6.5 0-8.5 1.5z"/><path d="M12 6.5V20"/>`,
  gear: `<circle cx="12" cy="12" r="3"/><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1"/>`,
  sort: `<path d="M8 5v14M4.5 15.5 8 19l3.5-3.5M16 19V5M12.5 8.5 16 5l3.5 3.5"/>`,
  swap: `<path d="M4 8h14M14.5 4.5 18 8l-3.5 3.5M20 16H6M9.5 12.5 6 16l3.5 3.5"/>`,
  spell: `<path d="M5 6.5h14M5 11h9"/><path d="M4 17.5c1.3-1.4 2.7-1.4 4 0s2.7 1.4 4 0 2.7-1.4 4 0 2.7 1.4 4 0"/>`,
  case: `<rect x="3.5" y="7.5" width="17" height="12" rx="1.5"/><path d="M9 7.5v-2a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M3.5 12.5h17"/>`,
  layers: `<path d="M12 3.5 20.5 8 12 12.5 3.5 8z"/><path d="m3.5 12 8.5 4.5 8.5-4.5M3.5 16l8.5 4.5 8.5-4.5"/>`,
  send: `<path d="M20.5 3.5 3.5 10.5l7 3 3 7z"/><path d="m10.5 13.5 10-10"/>`,
};
const picto = (name) => `<svg class="picto" viewBox="0 0 24 24" aria-hidden="true">${PICTO[name] || PICTO.arrow}</svg>`;

const TOPIC_PICTO = { the: "doc", "a-an": "letter", prepositions: "turn", verbs: "clock", structure: "list", punctuation: "comma", lexis: "swap", style: "pen", spelling: "spell" };
const WORK_PICTO = { standup: "person", "code-review": "chat", incident: "alert", architecture: "layers", interview: "case" };
const topicPicto = (id) => TOPIC_PICTO[id] || "doc";

// title and sub arrive as markup; callers escape anything the learner wrote.
// the glyph is always the arrow; only an answered card swaps it for its verdict.
function sign({ picto: p = "arrow", title, sub = "", figure = "", label = "", action = "", extra = "", long = false, flip = false }) {
  return `<section class="sign${long ? " long" : ""}${flip ? " flip" : ""}">
    <span class="tile">${picto(p)}</span>
    <div class="sign-text"><h1>${title}</h1>${sub ? `<p>${sub}</p>` : ""}</div>
    ${figure !== "" ? `<p class="sign-figure"><b>${figure}</b>${label ? `<span>${label}</span>` : ""}</p>` : ""}
    ${action ? `<div class="sign-action">${action}</div>` : ""}
    ${extra}
  </section>`;
}

const post = ({ href, picto: p, title, sub = "", quiet = false, current = false }) =>
  `<a class="post${quiet ? " quiet" : ""}" href="${href}"${current ? ' aria-current="page"' : ""}>
    <span class="tile">${picto(p)}</span>
    <span class="post-text"><b>${title}</b>${sub ? `<span>${sub}</span>` : ""}</span>
    <span class="edge">${picto("arrow")}</span>
  </a>`;

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
  renderHealth(cache.status);
  return cache.status;
}

function renderHealth(st) {
  const el = document.getElementById("health");
  if (!el || !st) return;
  const item = (ok, label, title = label) => `<span title="${esc(title)}"><i class="${ok ? "" : "off"}"></i>${esc(label)}</span>`;
  const whisperOk = st.whisper?.openrouter_ok || st.whisper?.local_ready;
  const backend = st.llm?.backend || "";
  el.innerHTML =
    item(st.llm?.ok, st.llm?.ok ? backend.split(" ")[0] || "reviewer" : "no reviewer", backend || "no reviewer") +
    item(whisperOk, "whisper") +
    `<span title="LLM spend since Monday: prompt checks and speech">$${(st.week_cost || 0).toFixed(2)} this week</span>`;
}

const topicTitle = (id) => cache.status?.topics.find((t) => t.id === id)?.title || id;
const when = (iso) => (iso ? new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short" }) : "");
const dayOf = (date) => new Date(date + "T12:00:00").toLocaleDateString("en-GB", { day: "numeric", month: "short" });
const times = (n) => (n === 1 ? "once" : n === 2 ? "twice" : `${n} times`);
const nth = (n) => `${n}${[11, 12, 13].includes(n % 100) ? "th" : ["th", "st", "nd", "rd"][n % 10] || "th"}`;
const plural = (n, one, many = one + "s") => `${n} ${n === 1 ? one : many}`;
const reviewMinutes = (cards) => Math.max(1, Math.round(cards * 0.5));

function nextAt(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  const time = d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
  return d.toDateString() === new Date().toDateString() ? time : `${when(iso)}, ${time}`;
}

// the text archive id says where a mistake was caught.
function sourceOf(textId = "") {
  if (/-prompts(-|$)/.test(textId)) return { short: "prompt", long: "from a prompt" };
  if (/-speaking(-|$)/.test(textId)) return { short: "speech", long: "from a spoken answer" };
  if (textId.includes("-web-")) return { short: "check", long: "from a checked text" };
  return { short: "text", long: "from a text" };
}

const BOXES = ["10 min", "1 day", "3 days", "7 days", "21 days", "60 days"];

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

function rateChart(weeks) {
  const pts = weeks.filter((w) => w.words > 0);
  if (pts.length < 2) {
    return pts.length
      ? `<p class="muted">${pts[0].rate.toFixed(1)} this week. The line starts after a second week of checked prompts.</p>`
      : `<p class="muted">This fills in as you write English prompts. It counts per week, so it drops as your English improves.</p>`;
  }
  const W = 520, H = 180, l = 28, r = 28, t = 30, b = 30;
  const top = Math.max(...pts.map((p) => p.rate)) * 1.15 || 1;
  const x = (i) => l + (i * (W - l - r)) / (pts.length - 1);
  const y = (v) => t + (1 - v / top) * (H - t - b);
  const path = pts.map((p, i) => `${i ? "L" : "M"}${x(i).toFixed(1)} ${y(p.rate).toFixed(1)}`).join(" ");
  const marks = pts
    .map(
      (p, i) => `<circle class="pt" cx="${x(i).toFixed(1)}" cy="${y(p.rate).toFixed(1)}" r="4.5"><title>${p.week}: ${p.rate.toFixed(1)} per 100 words over ${p.words} words</title></circle>
        <text x="${x(i).toFixed(1)}" y="${(y(p.rate) - 13).toFixed(1)}" text-anchor="middle">${p.rate.toFixed(1)}</text>
        <text class="axis" x="${x(i).toFixed(1)}" y="${H - 6}" text-anchor="middle">${dayOf(p.start)}</text>`
    )
    .join("");
  return `<svg class="rate-chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="Mistakes per 100 checked words, by week">
      <line class="base" x1="${l}" x2="${W - r}" y1="${H - b}" y2="${H - b}"/>
      <path class="area" d="${path} L${x(pts.length - 1).toFixed(1)} ${H - b} L${x(0).toFixed(1)} ${H - b} Z"/>
      <path class="line" d="${path}"/>${marks}</svg>`;
}

function rateFigures(st) {
  const pts = st.weeks.filter((w) => w.words > 0);
  const first = pts[0]?.rate, last = pts[pts.length - 1]?.rate;
  const change = pts.length > 1 && first ? Math.round(((last - first) / first) * 100) : null;
  const answers = st.weeks.reduce((n, w) => n + w.answers, 0);
  const right = st.weeks.reduce((n, w) => n + w.accuracy * w.answers, 0);
  return `<div class="figures">
      <div><b>${change === null ? "—" : `${change > 0 ? "+" : change < 0 ? "−" : "±"}${Math.abs(change)}%`}</b>${change === null ? "rate change" : `in ${plural(pts.length, "week")}`}</div>
      <div><b>${st.streak}</b>${st.streak === 1 ? "day in a row" : "days in a row"}</div>
      <div><b>${answers ? `${Math.round((right / answers) * 100)}%` : "—"}</b>drill accuracy</div>
    </div>`;
}

function firstRun(st) {
  const whisperOk = st.whisper?.openrouter_ok || st.whisper?.local_ready;
  const step = (n, text) => `<li><b>${n}</b>${text}</li>`;
  const arrow = `<li aria-hidden="true">${picto("arrow")}</li>`;
  const check = (ok, title, sub) =>
    `<li class="${ok ? "ok" : "warn"}"><span class="tile">${picto(ok ? "check" : "alert")}</span><div><b>${title}</b><div class="muted">${sub}</div></div></li>`;
  view.innerHTML = `
    ${sign({
      title: "Start here",
      extra: `<ol class="route" aria-label="Three steps">${step(1, "Install the hooks")}${arrow}${step(2, "Write a prompt in English")}${arrow}${step(3, "Come back for your first drill")}</ol>`,
    })}
    <div class="duo">
      <section class="panel">
        <h2>Setup</h2>
        <ul class="checklist">
          ${check(st.llm.ok, st.llm.ok ? `Reviewer ready: ${esc(st.llm.backend)}` : "No reviewer yet", st.llm.ok ? "Each English prompt is checked in the background." : "Set OPENROUTER_API_KEY or install the claude CLI.")}
          ${check(whisperOk, whisperOk ? "Speech to text ready" : "Speech to text not set up", whisperOk ? "Speak is ready to record." : "Run <code>eng whisper-setup</code> to use Speak.")}
          <li><span class="tile">${picto("gear")}</span><div><b>Agent hooks</b><div class="muted">Run <code>eng install</code> once; <code>eng doctor</code> shows what is still missing.</div></div></li>
        </ul>
      </section>
      <div class="posts stack">
        ${post({ href: "#/check", picto: "send", title: "No agent hooks? Paste English into Check", sub: "Same review, same mistake database" })}
        ${post({ href: "#/work", picto: "chat", title: "Practise work English", sub: "Stand-ups, code review, interviews" })}
        ${post({ href: "#/lessons", picto: "book", title: "Read the lessons", sub: "Articles, prepositions, verb forms and more" })}
      </div>
    </div>`;
}

async function today() {
  const [st, mistakes] = await Promise.all([getStatus(), getMistakes(true)]);
  if (!st.totals.mistakes) return firstRun(st);
  const hero = pickHero(st, mistakes);
  const heroTopic = hero?.entry.topics[0];
  const ripe = st.topics.filter((t) => t.ready).sort((a, b) => b.fresh - a.fresh);
  const waiting = st.topics.filter((t) => !t.ready && t.fresh > 0).sort((a, b) => b.fresh / b.threshold - a.fresh / a.threshold);

  let next, posts;
  if (st.due) {
    next = "#/review";
    posts = ripe.slice(0, 3);
  } else if (ripe.length) {
    next = `#/drill/${ripe[0].id}`;
    posts = ripe.slice(1, 4);
  } else {
    next = "#/check";
    posts = [];
  }
  posts = [...posts, ...waiting].slice(0, 4);

  const signHTML = st.due
    ? sign({
        title: "Review",
        sub: `${plural(st.due, "due card")} · about ${reviewMinutes(st.due)} min`,
        figure: st.due,
        label: "due now",
        action: `<a class="button primary" href="#/review">Start review <kbd>↵</kbd></a>`,
      })
    : ripe.length
      ? sign({
          title: `Practise ${esc(ripe[0].title)}`,
          sub: `mistakes since ${ripe[0].drills ? "your last drill" : "you started"} · ripe at ${ripe[0].threshold}`,
          figure: ripe[0].fresh,
          label: "new",
          action: `<a class="button primary" href="#/drill/${esc(ripe[0].id)}">Start <kbd>↵</kbd></a>`,
        })
      : sign({
          title: "Nothing is due",
          sub: st.next_due ? `Next review at ${nextAt(st.next_due)}` : "New cards arrive as you write English",
          action: `<a class="button primary" href="#/check">Check a text <kbd>↵</kbd></a>`,
        });

  const postsHTML = posts.length
    ? `<nav class="posts" aria-label="Topics">${posts
        .map((t) =>
          post({
            href: `#/drill/${esc(t.id)}`,
            picto: topicPicto(t.id),
            title: t.ready ? `Ripe: ${esc(t.title)}` : esc(t.title),
            sub: t.ready ? `${t.fresh} new${t.drills ? " since last drill" : ""}` : `${t.fresh} of ${t.threshold} · open to practise now`,
            quiet: !t.ready,
          })
        )
        .join("")}</nav>`
    : "";

  const heroHTML = hero
    ? `<section class="panel">
        <h2>Last caught</h2>
        <div class="paper"><p class="sentence hero">${renderDiff(hero.entry.before, hero.entry.after)}</p></div>
        <p class="meta">${esc(hero.rule.rule)} · ${nth(hero.rule.count)} time · last on ${dayOf(hero.rule.last)} · ${sourceOf(hero.entry.text_id).long}</p>
        ${hero.entry.note ? `<p class="meta">${esc(hero.entry.note)}</p>` : ""}
        <div class="actions">
          ${heroTopic ? `<a class="button" href="#/drill/${esc(heroTopic)}">Practise ${esc(topicTitle(heroTopic))}</a>` : ""}
          <button class="button" data-cheer>Cheer me up</button>
        </div>
      </section>`
    : `<section class="panel"><h2>Last caught</h2><p class="muted">Write to your agent in English and your corrections show up here.</p></section>`;

  const boxes = st.boxes || [];
  const alerts = [
    st.failed ? `<p class="alert">${plural(st.failed, "prompt check")} failed · run <code>eng check -retry</code></p>` : "",
    st.speaking.pending ? `<p class="alert"><a href="#/speak">${plural(st.speaking.pending, "spoken answer")} not checked yet</a></p>` : "",
    st.llm.ok ? "" : `<p class="alert">No reviewer for new prompts · run <code>eng doctor</code></p>`,
  ].join("");

  view.innerHTML = `
    ${signHTML}
    ${postsHTML}
    <div class="duo">
      ${heroHTML}
      <section class="panel">
        <h2>Mistakes per 100 checked words</h2>
        ${rateChart(st.weeks)}
        ${rateFigures(st)}
      </section>
    </div>
    <div id="growth"></div>
    <div class="strip">
      <section class="timetable" aria-label="Next reviews">
        <h2>Next reviews</h2>
        <ol>${BOXES.map((b, i) => `<li>${b} <b>${boxes[i] ?? 0}</b></li>`).join("")}</ol>
        ${st.next_due && !st.due ? `<span class="next">next at ${nextAt(st.next_due)}</span>` : ""}
      </section>
      ${alerts ? `<div class="alerts">${alerts}</div>` : ""}
    </div>
    ${st.top_rules.length
      ? `<section class="panel"><h2>Your most repeated mistakes</h2>
          <table class="board compact"><tbody>${st.top_rules
            .map((r) => `<tr><td class="rule">${esc(r.rule)}</td><td class="num">×${r.count}</td><td class="when">${dayOf(r.last)}</td></tr>`)
            .join("")}</tbody></table></section>`
      : ""}`;

  view.querySelector("[data-cheer]")?.addEventListener("click", (e) => growthCard(e.currentTarget));

  keyHandler = (e) => {
    if (e.key === "Enter" && document.activeElement === view) location.hash = next;
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
        const shown = (w) => esc(w);
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
  const fix = normGap(answer) === NONE ? `<ins class="fix" title="no word here">${NONE}</ins>` : `<ins class="fix">${esc(answer)}</ins>`;
  if (normGap(given) === normGap(answer)) return fix;
  return (normGap(given) !== NONE ? `<del class="slip">${esc(given)}</del> ` : "") + fix;
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
    const ripe = st.topics.find((t) => t.ready);
    view.innerHTML = `${sign({
        title: topic === "review" ? "Nothing is due right now" : "No exercises in this topic yet",
        sub: topic === "review" && st.next_due ? `Next review at ${nextAt(st.next_due)}` : "",
        action: ripe ? `<a class="button primary" href="#/drill/${esc(ripe.id)}">Practise ${esc(ripe.title)}</a>` : `<a class="button primary" href="#/">Back to today</a>`,
        long: true,
      })}
      <section class="leaf"><div class="text">
        <p class="prose">${topic === "review"
          ? "Cards come back 1, 3, 7, 21 and 60 days after you answer them correctly, and the same day when you miss them."
          : "Exercises appear as mistakes in this topic are logged, or when you ask for new ones."}</p>
        ${ripe ? `<div class="actions"><a class="button" href="#/">Back to today</a></div>` : ""}
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

  const isSingle = (c) => c.type === "cloze" && c.choices?.length && c.text.split("___").length === 2;

  const askSign = (c) =>
    sign({
      title: c.type === "fix" ? "Rewrite the sentence so it is correct" : isSingle(c) ? "Pick the word that fits" : "Type the missing words",
      sub: `${esc(title)}${isSingle(c) ? ` · keys 1 to ${c.choices.length}` : c.type === "fix" ? " · edit in place, then Enter" : ""}`,
      figure: idx + 1,
      label: `of ${cards.length}`,
      extra: ticks(),
      long: true,
      flip: idx > 0,
    });

  const answeredSign = (ok, selfGrade) =>
    sign({
      picto: selfGrade ? "help" : ok ? "check" : "alert",
      title: selfGrade ? "Was yours right?" : ok ? "Right" : "Not quite",
      sub: selfGrade ? "Close, but not word for word" : ok ? esc(title) : "The fix is marked in blue",
      figure: idx + 1,
      label: `of ${cards.length}`,
      action: selfGrade
        ? `<button class="primary" data-grade="1">Mine was right <kbd>y</kbd></button><button data-grade="0">I missed it <kbd>n</kbd></button>`
        : `<button class="primary" data-next>${idx + 1 < cards.length ? "Next card" : "Finish"} <kbd>↵</kbd></button>`,
      extra: ticks(),
      long: true,
      flip: true,
    });

  const askNote = (c) =>
    (c.type === "fix"
      ? `<strong>Fix the whole sentence.</strong><p>Grammar, articles and word order all count.</p>`
      : isSingle(c)
        ? `<strong>${NONE} means no word belongs there.</strong>`
        : `<strong>Leave a blank empty or type ${NONE} when no word belongs there.</strong>`) +
    `<p class="box-line">${boxLine(c)}</p>` +
    (summary ? `<p>${esc(summary)}</p>` : "");

  // grading moves a card one box up when right and back to the first box when missed.
  const boxOf = (c) => Math.min(Math.max(c.box || 0, 0), BOXES.length - 1);
  const backIn = (c, ok) => (ok ? BOXES[Math.min(boxOf(c) + 1, BOXES.length - 1)] : BOXES[0]);
  const boxLine = (c) => `Box ${boxOf(c) + 1} of ${BOXES.length} · right: back in ${backIn(c, true)} · missed: back in ${backIn(c, false)}`;

  const answerNote = (c, ok, selfGrade) => `<strong>${esc(c.rule)}</strong>
      <p class="box-line">${selfGrade ? boxLine(c) : `Back in ${backIn(c, ok)}`}</p>
      ${c.note ? `<p>${esc(c.note)}</p>` : ""}
      ${c.source === "mistake" && c.date ? `<p>From your writing on ${dayOf(c.date)}.</p>` : c.source === "pack" ? "<p>A new sentence written from your mistakes.</p>" : ""}`;

  function frame(signHTML, inner, note, below = "") {
    view.innerHTML = `${signHTML}
      <section class="leaf">
        <div class="text">${inner}</div>
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
      answeredSign(ok, selfGrade),
      `<div class="paper">${inner}${wroteHTML(c)}</div>
       <div class="actions" id="after">
         <button data-say>${SPEAKER}Hear it</button>
         <button data-explain>Explain this more</button>
       </div>`,
      answerNote(c, ok, selfGrade),
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
    const single = isSingle(c);
    const checkActions = `<div class="actions"><button class="primary" data-check>Check <kbd>↵</kbd></button><button class="link" data-skip>Show the answer</button></div>`;
    let inner;
    if (c.type === "fix") {
      inner = `<div class="paper"><p class="sentence medium">${esc(c.text)}</p>${wroteHTML(c)}</div>
        <textarea id="answer" rows="3" aria-label="Your corrected sentence" spellcheck="false">${esc(c.text)}</textarea>
        ${checkActions}`;
    } else {
      inner = `<div class="paper"><p class="sentence">${clozeHTML(c)}</p>${wroteHTML(c)}</div>
        ${single
          ? `<div class="choices">${c.choices.map((ch, k) => `<button data-choice="${esc(ch)}"><kbd>${k + 1}</kbd>${esc(ch)}</button>`).join("")}</div>`
          : checkActions}`;
    }
    frame(askSign(c), inner, askNote(c));

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
    view.innerHTML = `${sign({
        title: `${right} of ${results.length} right`,
        sub: esc(title),
        action: `<button class="primary" data-again>Another round <kbd>↵</kbd></button>`,
        extra: ticks(),
        long: true,
        flip: true,
      })}
      <section class="leaf">
      <div class="text">
        ${missed.length ? `<section class="panel"><h2>Worth another look</h2><ul class="plain">${missed.map((r) => `<li>${esc(r)}</li>`).join("")}</ul></section>` : ""}
        <div class="actions">
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
  return `<section class="panel">
      <h2>Corrected</h2>
      <div class="paper"><p class="sentence small">${renderDiff(rec.transcript, r.corrected)}</p></div>
      <div class="actions"><button data-hear="${esc(r.corrected)}">${SPEAKER}Hear the corrected version</button>${costBadge(rec)}</div>
    </section>
    ${errs ? `<section class="panel"><h2>What to fix</h2><ul class="plain">${errs}</ul></section>` : `<p class="verdict ok">No grammar mistakes found.</p>`}
    ${r.tips?.length ? `<section class="panel"><h2>Tips</h2><ul class="plain">${r.tips.map((t) => `<li>${esc(t)}</li>`).join("")}</ul></section>` : ""}
    ${r.fluency ? `<p class="prose">${esc(r.fluency)}</p>` : ""}`;
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
  return `<section class="panel"><h2>STAR review</h2>
    <table class="star-table"><tbody>${rows}</tbody></table>
    <p><strong>Overall:</strong> ${starBar(r.overall_score)}</p>
    ${r.rewrite ? `<h3 class="sub">Tighter version</h3><p class="sentence small">${esc(r.rewrite)}</p>` : ""}
    ${r.story_match ? `<p class="prose muted">Best story match: ${esc(r.story_match)}</p>` : ""}
    ${r.strengths?.length ? `<h3 class="sub">Strengths</h3><ul class="plain">${r.strengths.map((x) => `<li>${esc(x)}</li>`).join("")}</ul>` : ""}
    ${r.gaps?.length ? `<h3 class="sub">Gaps</h3><ul class="plain">${r.gaps.map((x) => `<li>${esc(x)}</li>`).join("")}</ul>` : ""}
    ${r.follow_ups?.length ? `<h3 class="sub">Likely follow-ups</h3><ul class="plain">${r.follow_ups.map((x) => `<li>${esc(x)}</li>`).join("")}</ul>` : ""}
    <div class="actions">${costBadge(rec)}</div></section>`;
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
        <div class="stack-gap"><div class="paper"><p class="sentence small">${esc(r.transcript)}</p></div>${r.interview_review ? interviewReviewHTML(r) : reviewHTML(r)}</div></details></li>`
    )
    .join("");

  const modeOption = (value, text) =>
    `<label><input type="radio" name="mode" value="${value}" ${reviewMode === value ? "checked" : ""}>${text}</label>`;

  view.innerHTML = `${sign({
      title: esc(q.prompt),
      sub: `Answer out loud · one to two minutes${q.focus ? ` · watch ${esc(q.focus)}` : ""}`,
      long: true,
    })}
    <section class="leaf">
      <div class="text">
        ${whisperOk ? "" : `<div class="callout">Recording works once these are in place: ${esc(tools.missing || "OpenRouter key or local whisper-cpp")}. Run <code>eng doctor</code> in a terminal, then reload.</div>`}
        ${st.llm.ok ? "" : `<div class="callout">The grammar check needs OpenRouter or the claude CLI; <code>eng doctor</code> shows which one is missing.</div>`}
        <section class="panel">
          <div class="recorder">
            <button class="record" id="rec" ${whisperOk ? "" : "disabled"}><span class="rec-dot">${picto("mic")}</span><span id="rec-label">Record</span> <kbd>space</kbd></button>
            <span class="timer" id="timer"></span>
            <fieldset class="segmented"><legend>Review mode</legend>${modeOption("grammar", "Grammar")}${modeOption("star", "STAR interview")}</fieldset>
            <button class="link" id="another">Another question</button>
          </div>
          ${backendNote}
        </section>
        <div id="out" class="stack-gap"></div>
      </div>
      <aside class="note"><strong>Talk the way you would at work.</strong>
        <p>Whisper writes down what you say; the check ignores fillers and punctuation.</p>
        <p>Grammar marks what to fix. STAR rates an interview answer by situation, task, action and result.</p></aside>
    </section>
    <section class="stack-gap"><h2>Earlier answers</h2>${hist ? `<ul class="plain">${hist}</ul>` : `<p class="muted">Your recorded answers will be listed here.</p>`}</section>`;
  wireHear(view);

  const out = view.querySelector("#out");
  const recBtn = view.querySelector("#rec");
  const label = view.querySelector("#rec-label");
  const timer = view.querySelector("#timer");
  view.querySelectorAll('input[name="mode"]').forEach((r) => r.addEventListener("change", () => { reviewMode = r.value; }));
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
      out.innerHTML = `<section class="panel"><h2>What Whisper heard</h2>
        <div class="paper"><p class="sentence small">${esc(rec.transcript) || "<span class='muted'>Nothing was recognised.</span>"}</p></div>
        <div class="actions"><button class="primary" id="check" ${canCheck ? "" : "disabled"}>${reviewMode === "star" ? "Check as STAR answer" : "Check my grammar"}</button>
        <span class="muted">${rec.words} words · ${esc(rec.whisper_backend)}</span></div></section><div id="review" class="stack-gap"></div>`;
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

  const nav = `<nav class="posts stack nav" aria-label="Work English topics">${cats
    .map((c) =>
      post({
        href: `#/work/${esc(c.id)}`,
        picto: WORK_PICTO[c.id] || "chat",
        title: esc(c.title),
        sub: `${plural(c.phrases?.length || 0, "phrase")}${c.prompts?.length ? ` · ${plural(c.prompts.length, "prompt")}` : ""}`,
        current: c.id === active?.id,
      })
    )
    .join("")}</nav>`;

  const categoryHTML = (c) => `
    ${sign({
      title: esc(c.title),
      sub: c.id === "interview" ? "Phrases to borrow, then answer out loud for a STAR review" : "Phrases to borrow, then answer out loud for a grammar check",
      long: true,
    })}
    ${c.phrases?.length
      ? `<section class="panel"><h2>Useful phrases</h2><ul class="phrases">${c.phrases
          .map((p) => `<li><div><p>${esc(p.text)}</p>${p.notes ? `<p class="muted">${esc(p.notes)}</p>` : ""}</div>
            <button data-hear="${esc(p.text)}">${SPEAKER}Listen</button></li>`)
          .join("")}</ul></section>`
      : ""}
    ${c.prompts?.length
      ? `<section class="panel"><h2>Practice prompts</h2><ul class="prompts">${c.prompts
          .map((p) => `<li><p>${esc(p)}</p><a class="button primary" href="#/speak?mode=${c.id === "interview" ? "star" : "grammar"}&prompt=${encodeURIComponent(p)}">${picto("mic")}Record an answer</a></li>`)
          .join("")}</ul></section>`
      : ""}`;

  view.innerHTML = active
    ? `<div class="split">${nav}<div class="split-main">${categoryHTML(active)}</div></div>`
    : `<p class="muted">No Work English content yet.</p>`;
  wireHear(view);
}

/* ---------- check a pasted text ---------- */

const KIND_LABEL = { grammar: "grammar", punctuation: "punct", lexical: "word choice", spelling: "spelling", style: "style" };

async function checkView() {
  const st = cache.status ? cache.status : await getStatus();
  const mod = /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘" : "Ctrl";
  const placeholder = `<h2>Corrected</h2><p class="muted">The corrected text appears here. What you confirm joins the mistake pile and comes back as drills; style notes are advice and never enter it.</p>`;
  view.innerHTML = `${sign({
      title: "Check a text before you send it",
      sub: "An email, an MR description or a message · nothing is logged until you confirm",
      long: true,
    })}
    <div class="duo even">
      <section class="panel">
        <h2>Your text</h2>
        <textarea id="paste" rows="10" placeholder="Paste your English here…" spellcheck="false" aria-label="Text to check"></textarea>
        <div class="actions">
          <button class="primary" id="run" disabled>Check <kbd>${mod} ↵</kbd></button>
          <button class="link" id="clear" hidden>Clear</button>
          <span class="muted" id="meta"></span>
        </div>
        ${st.llm.ok ? "" : `<div class="callout">No reviewer is set up: run <code>eng doctor</code> in a terminal.</div>`}
      </section>
      <section class="panel" id="out" aria-live="polite">${placeholder}</section>
    </div>
    <div id="fixes" class="stack-gap"></div>`;
  const ta = view.querySelector("#paste");
  const run = view.querySelector("#run");
  const clearBtn = view.querySelector("#clear");
  const out = view.querySelector("#out");
  const fixes = view.querySelector("#fixes");
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
    out.innerHTML = placeholder;
    fixes.innerHTML = "";
    meta.textContent = "";
    draft = null;
    explainers.clear();
    ta.focus();
  });

  const mistakeRow = (e, count, k) => `<li>
      <p class="diff">${renderDiff(e.before, e.after)}</p>
      <div class="why"><strong>${esc(e.rule)}</strong>
        <p><span class="tag kind">${esc(KIND_LABEL[e.category] || e.category)}</span>${count ? ` ×${count} before` : ""}${e.note ? ` · ${esc(e.note)}` : ""}</p>
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
    out.innerHTML = `<h2>Corrected</h2><p class="muted wait">Reading your text… <span data-elapsed>0 s</span></p>`;
    fixes.innerHTML = "";
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
      out.innerHTML = `<h2>Corrected</h2><p class="error">${esc(e.message)}</p>`;
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
      <h2>Corrected</h2>
      <div class="paper"><p class="sentence small">${draft.corrected === draft.original ? esc(draft.corrected) : renderDiff(draft.original, draft.corrected)}</p></div>
      <div class="actions">
        <button data-hear="${esc(draft.corrected)}">${SPEAKER}Hear it</button>
        ${draft.cost ? `<span class="cost">${esc(draft.backend || "")} $${draft.cost.toFixed(4)}</span>` : ""}
      </div>
      ${draft.perception ? `<h3 class="sub">How it reads</h3><p class="prose">${esc(draft.perception)}</p>` : ""}
      ${draft.native ? `<h3 class="sub">How a native would write it</h3>
      <div class="paper"><p class="sentence small">${esc(draft.native)}</p></div>
      <div class="actions"><button data-hear="${esc(draft.native)}">${SPEAKER}Hear it</button></div>` : ""}`;
    fixes.innerHTML = `
      ${fixable.length
        ? `<h2>What to fix <span class="muted">${fixable.length}</span></h2><ul class="fixes">${fixable.map(({ e, k }) => mistakeRow(e, counted[e.rule], k)).join("")}</ul>
           <div class="actions"><button class="primary" id="log">Add ${plural(fixable.length, "mistake")} to the pile</button></div>`
        : `<p class="verdict ok">No grammar mistakes found.</p>`}
      ${style.length
        ? `<h2>Style notes <span class="muted">advice, not logged</span></h2><ul class="fixes">${style.map(({ e, k }) => mistakeRow(e, 0, k)).join("")}</ul>` : ""}`;
    wireHear(out);
    wireHear(fixes);
    wireExplainers();
    const logBtn = fixes.querySelector("#log");
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
    section.className = "leaf growth";
    section.innerHTML = `
      <div class="text">
        <h2>Are you actually getting better?</h2>
        <div class="figures" aria-busy="true"><div class="skel"></div><div class="skel"></div><div class="skel"></div><div class="skel"></div></div>
      </div>`;
    const slot = document.getElementById("growth");
    if (slot) slot.replaceChildren(section);
    else anchor.closest("section").after(section);
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
  let newest = true;
  const oldest = list.reduce((d, m) => (m.date < d ? m.date : d), list[0]?.date || "");
  view.innerHTML = `${sign({
      title: list.length ? `${plural(list.length, "mistake")} · ${plural(st.totals.rules, "rule")}` : "No mistakes yet",
      sub: list.length ? `since ${dayOf(oldest)} · everything you wrote wrong, corrected` : "They arrive as your English prompts are checked",
      action: st.due ? `<a class="button primary" href="#/review">Review ${st.due} due</a>` : "",
      long: true,
    })}
    <div class="toolbar">
      <input type="search" id="q" placeholder="Search sentences or rules" aria-label="Search mistakes">
      <div class="filters" role="group" aria-label="Filter by topic">
        <button aria-pressed="true" data-topic="">All<span>${list.length}</span></button>
        ${st.topics.filter((t) => t.total).map((t) => `<button aria-pressed="false" data-topic="${esc(t.id)}">${esc(t.title)}<span>${t.total}</span></button>`).join("")}
      </div>
    </div>
    <div class="board-wrap sheet">
      <table class="board archive">
        <thead><tr>
          <th scope="col" aria-sort="descending"><button data-sort>Date ${picto("sort")}</button></th>
          <th scope="col">Before → after</th><th scope="col">Category</th><th scope="col">Rule</th><th scope="col">Source</th>
        </tr></thead>
        <tbody id="list"></tbody>
      </table>
    </div>`;
  const tbody = view.querySelector("#list");
  const sortTh = view.querySelector("th[aria-sort]");
  view.querySelector("[data-sort]").addEventListener("click", () => {
    newest = !newest;
    sortTh.setAttribute("aria-sort", newest ? "descending" : "ascending");
    draw();
  });
  function draw() {
    const q = query.toLowerCase();
    const rows = list.filter(
      (m) => (!topic || m.topics.includes(topic)) && (!q || `${m.before} ${m.after} ${m.rule}`.toLowerCase().includes(q))
    );
    if (!newest) rows.reverse();
    tbody.innerHTML = rows.length
      ? rows
          .slice(0, 300)
          .map(
            (m) => `<tr><td class="when">${dayOf(m.date)}</td><td class="diff">${renderDiff(m.before, m.after)}</td>
              <td class="cat">${esc(m.category)}</td><td class="rule">${esc(m.rule)}${m.note ? `<small>${esc(m.note)}</small>` : ""}</td>
              <td class="src">${sourceOf(m.text_id).short}</td></tr>`
          )
          .join("")
      : `<tr><td colspan="5" class="muted">Nothing matches.</td></tr>`;
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
  const nav = `<nav class="posts stack nav" aria-label="Topics">${st.topics
    .map((t) =>
      post({
        href: `#/lessons/${esc(t.id)}`,
        picto: topicPicto(t.id),
        title: esc(t.title),
        sub: t.total ? `${plural(t.total, "mistake")} of yours` : "none of yours yet",
        current: t.id === topic,
      })
    )
    .join("")}</nav>`;
  const practise = `<a class="button primary" href="#/drill/${esc(topic)}">Practise ${esc(topicTitle(topic))}</a>`;
  const body = lesson
    ? `${sign({ title: esc(lesson.summary), sub: esc(topicTitle(topic)), action: practise, long: true })}
      ${lesson.rules
        .map(
          (r) => `<section class="panel"><h2>${esc(r.title)}</h2><p class="rule-body">${esc(r.body)}</p>
            ${(r.examples || []).map((x) => `<p class="example">${renderDiff(x.bad, x.good)}</p>`).join("")}</section>`
        )
        .join("")}`
    : `${sign({ title: esc(topicTitle(topic)), sub: "No lesson written for this topic yet", action: practise, long: true })}`;
  const yours = mine.length
    ? `<section class="panel"><h2>From your own writing</h2><ul class="papers">${mine
        .map((m) => `<li class="paper"><p class="sentence small">${renderDiff(m.before, m.after)}</p><p class="muted">${esc(m.rule)} · ${dayOf(m.date)}</p></li>`)
        .join("")}</ul></section>`
    : "";
  view.innerHTML = `<div class="split">${nav}<div class="split-main">${body}${yours}</div></div>`;
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
    view.innerHTML = `<section class="panel"><h2>Something broke while loading this page</h2>
      <p class="error">${esc(e.message)}</p><p class="muted">Run <code>eng doctor</code> in a terminal to see what is missing.</p></section>`;
  }
  view.focus({ preventScroll: true });
  window.scrollTo(0, 0);
}

window.addEventListener("hashchange", route);
route();
