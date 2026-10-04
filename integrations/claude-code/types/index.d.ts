// The hydra mod's session state. Values live in $.state so a hot reload keeps
// them: which rules each loop has been given, which were held and when, what
// fired this turn, and what the doctors last reported.

// Rule file → the step a hold delivered it in, `<turnId>:<index>`.
export type HydraHolds = { [file: string]: string }

export type HydraFailure = {
  key: string
  scope: string
  name: string
  severity: 'error' | 'warning'
  fix: string
  hint?: string
}

declare module 'claude-code' {
  interface PluginState {
    hydra: {
      // Per loop ('main' or a subagent's id): rule files whose text it has had.
      loaded: Record<string, string[]>
      // Per loop: the holds made in it.
      held: Record<string, HydraHolds>
      // Per loop: the step the loop is on, `<turnId>:<index>`.
      step: Record<string, string>
      // Rule names that fired this turn, newest first.
      fired: string[]
      // What the doctors last reported failing.
      failures: HydraFailure[]
      // When the doctors last ran, on the session clock; -1 for never.
      doctorAt: number
      // The ability the last routed prompt went to, '' for none.
      lastAbility: string
    }
  }
}
