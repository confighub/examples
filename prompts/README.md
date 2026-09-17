# Prompts

Public prompts you can run against your own systems. They are analysis tools, not
demos: you paste one into an agentic coding tool that has shell access to something
you own, and you get back numbers about your own estate.

Nothing here talks to ConfigHub, and nothing here needs an account.

## Available

- [`config-repo-ten-questions.md`](./config-repo-ten-questions.md) — From the blog post "Four roads to config hell", this prompt helps your AI ask questions
  about your configuration repository: looking at Helm values, Argo CD ApplicationSets, Flux
  Kustomizations, Kustomize overlays or similar. What is allowed to drift, how
  variety is declared, how far one edit reaches, and what is half promoted.

## How to run one

Open a clone of the repository you want to look at, start Claude Code, Cursor,
Codex or any agentic tool with shell access, and paste the prompt. The tool writes
and runs its own scripts and reports what they returned.

## The rules every prompt here follows

Contributions are welcome, and they need to hold to these.

1. **Measure, never estimate.** The model writes and runs scripts and reports only
   what the scripts returned. A language model asked to eyeball a number will
   confidently produce one it never computed. This is the rule that makes the
   output worth reading, and it is the one that must not be softened.
2. **Say NOT OBSERVED.** If something cannot be answered from what the tool can
   see, it says so and says what it would need. A gap named is more useful than a
   gap filled in.
3. **Findings are questions, not defects.** Some of what these prompts surface is
   deliberate. The output asks the owner about it rather than marking it wrong.
4. **State the scope.** Which directories were covered, which definitions could not
   be resolved.
5. **No product in the output.** No ConfigHub, no score, no verdict, no dollar
   estimate. A prompt whose every answer points at a vendor is a sales sheet, and
   nobody runs a sales sheet against their own repository twice.

## Reading the output

The interesting answers are usually the ones nobody chose. A default left in
place, a values file that has never resolved, a version string repeated forty
times. None of those was a decision, and that is what makes them worth a
conversation.
