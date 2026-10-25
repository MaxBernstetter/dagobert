# Development principles

How to plan and implement a piece of software in Dagobert: a job, a feature, a fix. The principles apply to humans and coding agents alike. Adapted from the yield-lens development principles; the job contract and test rules of this repository are in [04-coding-principles](../04-coding-principles.md) and take precedence where they overlap.

## Workflow at a glance

1. **Plan:** check applicability, then cut the smallest slice ([02-applicability](02-applicability.md), [03-simplicity](03-simplicity.md)).
2. **Design:** respect the stateless job contract and the twelve factors ([01-twelve-factor](01-twelve-factor.md)).
3. **Implement:** clean code, readable first ([04-clean-code](04-clean-code.md)).
4. **Validate:** prove it works, at the right test level ([05-testing](05-testing.md)).
5. **Check scale:** confirm the design holds at 100× ([06-scalability](06-scalability.md)).
6. **Ship safely:** Compose, Taskfile and security rules ([07-delivery-and-security](07-delivery-and-security.md)).
7. **Finish:** check the [Definition of done](#definition-of-done), then close the task ([01-task-management](../01-task-management.md)).

## Principles

1. [01-twelve-factor](01-twelve-factor.md): building services that run the same locally and deployed
2. [02-applicability](02-applicability.md): does this piece solve the actual problem, here and now?
3. [03-simplicity](03-simplicity.md): the simplest thing that works, and no more
4. [04-clean-code](04-clean-code.md): code that reads like the domain
5. [05-testing](05-testing.md): how to show a piece of software works
6. [06-scalability](06-scalability.md): designing for 100× without building for it today
7. [07-delivery-and-security](07-delivery-and-security.md): build, run and security rules

## Definition of done

- [ ] Solves the issue's problem, and acceptance criteria are met
- [ ] Simplest viable design; no speculative features
- [ ] Twelve-factor compliant: config from environment, no secrets in the repo, job stateless
- [ ] Every acceptance criterion is covered by a test ([05-testing](05-testing.md))
- [ ] `task test` passes for the touched jobs; `npm run build` passes for frontend changes
- [ ] Rerunning the change is safe: stored work exits `2` (or `DETAIL=1` no-op), no second order
- [ ] No design choice blocks 100× scale
- [ ] Docs updated; diff reviewed and within the pull request size limit in [01-task-management](../01-task-management.md)
