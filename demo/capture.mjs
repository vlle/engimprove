// records the web app on a demo root: an animated GIF plus light/dark stills
import { execFileSync, spawn } from "node:child_process";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import puppeteer from "puppeteer-core";

const root = resolve(process.env.ENG_DEMO_ROOT ?? "demo/.root");
const bin = resolve("bin/eng");
const assets = resolve("demo/assets");
const frames = resolve("demo/.frames");
const chrome = process.env.CHROME ?? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const base = "http://127.0.0.1:7422";

const log = (msg) => console.error(`${new Date().toTimeString().slice(0, 8)} ${msg}`);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function waitForServer() {
  for (let i = 0; i < 50; i++) {
    try {
      if ((await fetch(`${base}/api/ping`)).ok) return;
    } catch {}
    await sleep(100);
  }
  throw new Error("server did not come up on " + base);
}

rmSync(frames, { recursive: true, force: true });
mkdirSync(frames, { recursive: true });
mkdirSync(assets, { recursive: true });

const server = spawn(bin, ["serve"], { env: { ...process.env, ENG_ROOT: root }, stdio: "ignore" });
let browser;
try {
  await waitForServer();
  browser = await puppeteer.launch({ executablePath: chrome, headless: true, args: ["--no-sandbox"] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1200, height: 760, deviceScaleFactor: 1 });
  await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: "light" }]);

  const list = [];
  let n = 0;
  const snap = async (hold) => {
    const file = `f${String(++n).padStart(4, "0")}.png`;
    await page.screenshot({ path: `${frames}/${file}` });
    list.push([file, hold]);
  };
  const burst = async (count, gap = 90) => {
    for (let i = 0; i < count; i++) {
      await snap(gap);
      await sleep(gap);
    }
  };
  const go = async (hash, ready) => {
    await page.evaluate((h) => (location.hash = h), hash);
    await page.waitForSelector(ready, { timeout: 8000 });
    await page.evaluate(() => document.fonts.ready);
    await sleep(250);
  };

  log("web: today");
  await page.goto(`${base}/#/`, { waitUntil: "networkidle0" });
  await page.waitForSelector("a.button.primary");
  await page.evaluate(() => document.fonts.ready);
  await sleep(300);
  await snap(2400);

  let deck = [];
  page.on("response", async (res) => {
    if (res.url().includes("/api/drill")) deck = (await res.json()).cards;
  });

  log("web: drill");
  await page.click("a.button.primary");
  await page.waitForSelector("[data-choice], [data-skip]", { timeout: 8000 });
  await sleep(300);
  let missOne = true;
  for (let card = 0; card < 4; card++) {
    await snap(1100);
    const choices = await page.$$("[data-choice]");
    if (choices.length) {
      const options = await page.$$eval("[data-choice]", (keys) => keys.map((k) => k.dataset.choice));
      const right = deck[card]?.answers?.[0];
      let pick = right && options.includes(right) ? right : options[0];
      if (missOne && card === 1) {
        pick = options.find((o) => o !== right) ?? pick;
        missOne = false;
      }
      await page.click(`[data-choice="${pick}"]`);
    } else {
      await page.click("[data-skip]");
    }
    await page.waitForSelector("[data-next], [data-grade], [data-again]", { timeout: 8000 });
    await sleep(200);
    await snap(1900);
    const next = await page.$("[data-next]");
    const grade = await page.$("[data-grade]");
    if (next) await next.click();
    else if (grade) await grade.click();
    else break;
    await sleep(300);
  }
  await page.waitForSelector("[data-again]", { timeout: 8000 }).catch(() => {});
  await sleep(300);
  await snap(2200);

  log("web: mistakes");
  await go("#/mistakes", "[data-topic]");
  await snap(2200);
  await page.click('[data-topic="the"]');
  await sleep(300);
  await snap(2200);

  log("web: back to today");
  await go("#/", "a.button.primary");
  await snap(1800);

  log("encoding gif");
  const concat = list.map(([f, ms]) => `file '${frames}/${f}'\nduration ${(ms / 1000).toFixed(3)}`).join("\n");
  writeFileSync(`${frames}/list.txt`, `${concat}\nfile '${frames}/${list.at(-1)[0]}'\n`);
  execFileSync("ffmpeg", [
    "-v", "error", "-y", "-f", "concat", "-safe", "0", "-i", `${frames}/list.txt`,
    "-vf", "fps=12,scale=960:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=96[p];[b][p]paletteuse=dither=bayer:bayer_scale=4",
    `${assets}/web.gif`,
  ]);

  log("stills");
  await page.setViewport({ width: 1200, height: 760, deviceScaleFactor: 2 });
  const stills = [
    ["today", "#/", "a.button.primary"],
    ["drill", "#/drill/the", "[data-choice], [data-skip]"],
    ["mistakes", "#/mistakes", "[data-topic]"],
  ];
  for (const scheme of ["light", "dark"]) {
    await page.emulateMediaFeatures([{ name: "prefers-color-scheme", value: scheme }]);
    for (const [name, hash, ready] of stills) {
      if (scheme === "dark" && name === "mistakes") continue;
      await go(hash, ready);
      await page.screenshot({ path: `${assets}/web-${name}-${scheme}.png` });
    }
  }
} finally {
  await browser?.close();
  server.kill();
  rmSync(frames, { recursive: true, force: true });
}
log("done");
