<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan at
`specs/001-flash-sale-reservation/plan.md` and its companion design artifacts
(`research.md`, `data-model.md`, `contracts/api.md`, `quickstart.md`) in the
same directory. The constitution at `.specify/memory/constitution.md` is the
governing document for technical decisions on this project.
<!-- SPECKIT END -->


# Claude Guidelines for beeyond-flash-sale

## 1. Plan Before Acting

Always enter plan mode before making any code changes. Present a clear implementation plan and wait for explicit approval before modifying any files.

- Use `EnterPlanMode` for all non-trivial tasks
- Do **not** edit, create, or delete files unless the user explicitly says to proceed (e.g., "go ahead", "do it", "implement it")
- If clarification is needed, ask before planning

## 2. Use Agent Teams for Research

When a task involves research, exploration, or gathering information across multiple areas, delegate to specialized subagents in parallel rather than doing it sequentially in the main context.

Examples of research tasks that warrant agent teams:
- Scanning the codebase for patterns across multiple files
- Comparing multiple migration strategies
- Investigating Postgres compatibility or type mappings

Use `subagent_type: "Explore"` for codebase exploration and `subagent_type: "general-purpose"` for broader research. Launch multiple agents concurrently when the subtasks are independent.

## 3. Warn Before Destructive Operations

Before running any operation that is hard to reverse or could cause data loss, explicitly state what the operation is and ask for confirmation. Do **not** proceed until confirmed.

Destructive operations include (but are not limited to):
- Dropping or truncating Postgres tables
- Deleting or overwriting migration files
- Running `DELETE` or `TRUNCATE` SQL statements
- Force-pushing git branches
- Removing dependencies or configuration files
- Any bulk data modification on live data

## 4. Architectural Governance (No Assumptions)
Never assume the architecture of a feature. Before generating React components or complex logic, you must explicitly state your architectural approach in the plan.
- Default to React Server Components unless interactivity (`useState`, `onClick`, hooks) strictly requires a Client Component.
- Explicitly mention how you will handle state management and data fetching.
- Do not generate code until the user approves the architectural approach.


## 5. Scope Restraint (Zero Hallucinations)
Stick strictly to the user's explicit requirements
- Do not invent or inject extra features, buttons, text, or aesthetic elements that were not explicitly requested.
- Prioritize functional completion and passing tests over unsolicited UI polish. 
- If a requirement is ambiguous, ask for clarification instead of guessing.


## 7. Safe File System & Build Operations
When configuring build tools (Tailwind, Webpack, Vite, etc.) or executing terminal commands:
- **Never use unconstrained glob patterns:** When defining file paths or "content" arrays, never use naked wildcards like `./**/*.{js,ts}`. You must strictly constrain paths to the actual source folders (e.g., `./src/**/*.{js,ts,jsx,tsx}`) and ensure `node_modules` is inherently bypassed.
- **Verify Execution Directory:** Before running commands like `npm run build` or `npx tailwindcss`, explicitly verify your Current Working Directory to ensure you are not running project-level commands from the user root directory.