// engimprove for opencode: queue every user prompt for the background check and
// notify about corrections when the session goes idle. `eng install` sets the eng
// path below and puts this file in ~/.config/opencode/plugins/.
// Feedback arrives as a desktop notification; opencode plugins cannot inject
// system messages into the conversation.

export const EngimprovePlugin = async ({ directory }) => {
  const eng = "__ENGBIN__"

  const spawn = (args, opts) => {
    try {
      const Bun = globalThis.Bun
      if (!Bun?.spawnSync) return ""
      const res = Bun.spawnSync([eng, ...args], opts)
      if (opts?.stdout !== "pipe") return ""
      return res.stdout ? res.stdout.toString() : ""
    } catch {}
    return ""
  }

  let lastSession = ""

  return {
    "chat.message": async (input, output) => {
      try {
        if (input?.sessionID) lastSession = input.sessionID
        const text = (output?.parts ?? [])
          .filter((p) => p?.type === "text")
          .map((p) => p?.text ?? "")
          .join("\n")
        if (!text.trim()) return
        spawn(["hook", "-session", lastSession || "nosession",
          "-cwd", directory ?? process.cwd(), "-text", text])
      } catch {}
    },
    event: async ({ event }) => {
      try {
        if (event?.type !== "session.idle") return
        const props = event?.properties ?? {}
        const sid = props.info?.id ?? props.info?.sessionID ?? props.sessionID ?? props.id ?? lastSession
        if (!sid) return
        const raw = spawn(["hook-stop", "-session", sid], { stdout: "pipe" })
        const msg = JSON.parse(raw)?.systemMessage
        if (msg) spawn(["notify", "-message", msg])
      } catch {}
    },
  }
}
