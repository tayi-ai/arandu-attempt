# Upgrade Guide

## v0.2.0

### The attempt is a concrete type over the non-generic model

Hesape `v0.47.0` removes the generic model layer, and this release moves to it
with Hesape `v0.48.0` and Framework `v0.50.2`. `Attempt` embeds the non-generic
`model.Model`, its table is declared once beside it with `model.NewTable`, and
the query that starts from it is generated beside it by `aru model:build`, in
`AttemptQuery.go`. No route, migration, action, policy decision or tenant rule
changed.

**`Attempts` returns the generated query.** It takes a `model.DB` -- a
`*data.DB` passes unchanged -- and returns `*attempt.AttemptQuery` instead of
`*model.Model[attempt.Attempt]`. A chain that started from it keeps its text,
minus the calls that no longer exist:

| before | now |
|---|---|
| `attempt.Attempts(db).NewQuery().Where(…)` | `attempt.Attempts(db).Where(…)` |
| `attempt.Attempts(db).NewInstance(nil, false)` and `.Entity` | `attempt.Attempts(db).New()`, which returns `*Attempt` |
| `Get` → `model.Collection[attempt.Attempt]` | `Get` → `attempt.AttemptCollection` (`[]*Attempt`) |
| `func(q *model.Builder[attempt.Attempt])` in a grouped `Where` | `func(q *attempt.AttemptQuery)` |
| `attempt.Attempts(db).GetTable()`, `.KeyType`, `.TenantColumn` | nothing: the table is unexported, and its settings are not read off the query |

`First`, `Find` and the other row terminals still return `*Attempt`, and still
take the Grant. Every `AttemptService` method keeps its signature: `Create` and
`Find` still return `*Attempt`, and `List` still returns `[]*Attempt`.

**The entity no longer carries the model's configuration.** `Attempt` embeds
`model.Model`, so the fields and methods `model.Model[Attempt]` promoted onto it
are gone: the configuration fields (`PrimaryKey`, `KeyType`, `Incrementing`,
`Timestamps`, `TenantColumn`, `Table` and the rest) live in the table, which this
package keeps unexported, and `Exists` and `WasRecentlyCreated` are methods,
`row.Exists()`. A copied row still reads its fields but refuses every write with
`model.ErrUnwired`, so keep the pointers the queries return.

**Upgrade the floor.** The module requires Hesape `v0.48.0` and Framework
`v0.50.2`, and `arandu.mod.toml` declares `framework = ">= 0.50"`. An
application that pins a Hesape below `v0.47.0` cannot compile this release:
every generic model type it would need is gone from Hesape itself.

**What changes without a compiler error.** The store route, `POST` on the
prefix, reads `name` through `Context.Input`, and since Hesape `v0.44.0` the
input of a `POST` is its body alone: a url-encoded or multipart form, or a JSON
object. A `name` sent only in the query string of the `POST` is no longer read,
and the request is refused by validation as one with no name. The listing and
the record routes are `GET` and read the query string as before.

<details>
<summary>Every incompatible symbol <code>apidiff</code> reports against v0.1.3</summary>

Most of these are the methods and fields `model.Model[Attempt]` promoted onto
`Attempt`, which left with the generic type.

```text
Attempt.ConnectionName
Attempt.CreatedAtColumn
Attempt.DeletedAtColumn
Attempt.Entity
Attempt.Exists
Attempt.Grammar
Attempt.Incrementing
Attempt.KeyType
Attempt.NamedScopes
Attempt.PerPage
Attempt.PrimaryKey
Attempt.Processor
Attempt.RelationResolvers
Attempt.SoftDeletes
Attempt.Table
Attempt.TenantColumn
Attempt.Timestamps
Attempt.UpdatedAtColumn
Attempt.WasRecentlyCreated
Attempts
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).AddGlobalScope, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).All, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Append, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).AttributesToArray, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).CallNamedScope, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Create, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Destroy, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).DiscardChanges, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Except, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Find, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).FindMany, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).FindOrFail, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).FindOrNew, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).First, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).FirstOrCreate, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).FirstOrNew, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ForceCreate, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ForceDeleteQuietly, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ForceDeleted, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ForceDeleting, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ForceDestroy, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).FreshTimestamp, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetAppends, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetConnectionName, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetCreatedAtColumn, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetDeletedAtColumn, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetForeignKey, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetGlobalScopes, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetHidden, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetIncrementing, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetKeyName, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetKeyType, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetMorphClass, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetPerPage, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetPrevious, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetQualifiedCreatedAtColumn, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetQualifiedDeletedAtColumn, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetQualifiedKeyName, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetQualifiedUpdatedAtColumn, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetQueueableConnection, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetQueueableID, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetQueueableRelations, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetRawOriginal, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetRelation, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetRelations, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetRouteKey, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetRouteKeyName, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetTable, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetTouchedRelations, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetUpdatedAtColumn, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).GetVisible, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).HasAppended, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).HasGlobalScope, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).HasNamedScope, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Is
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).IsForceDeleting, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).IsIgnoringTouch, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).IsNot, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).IsRelation, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).IsSoftDeletable, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadAggregate, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadMorph, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadMorphAggregate, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadMorphAvg, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadMorphCount, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadMorphMax, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadMorphMin, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).LoadMorphSum, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).MakeHidden
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).MakeVisible
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewBaseQueryBuilder, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewCollection, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewFromBuilder, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewInstance, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewModelQuery, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewQuery, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewQueryForRestoration, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewQueryWithoutRelationships, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewQueryWithoutScope, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewQueryWithoutScopes, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).NewTypedBuilder, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).On, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).OnWriteConnection, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Only, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).OnlyTrashed, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).OriginalIsEquivalent, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).PushQuietly, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).QualifyColumn, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).QualifyColumns, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Query, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Ref, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).RegisterGlobalScopes, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).RegisterModelEvent, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ReplicateQuietly, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ResolveRouteBinding, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ResolveRouteBindingQuery, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ResolveSoftDeletableRouteBinding, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).RestoreQuietly, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Restored, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Restoring, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetAppends, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetConnection, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetHidden, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetIncrementing, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetKeyName, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetKeyType, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetPerPage, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetRelation
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetRelations, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetTable, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetTouchedRelations, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SetVisible, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SoftDeleted, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SyncChanges, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SyncOriginal
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SyncOriginalAttribute, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).SyncOriginalAttributes, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).ToPrettyJSON, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Touches, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UnsetAttribute, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UnsetRelation, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UnsetRelations, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UpdateOrCreate, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UpdateOrFail, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UpdateQuietly, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UpdateTimestamps, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).UsesTimestamps, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).Where, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).WhereKey, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).With, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).WithTrashed, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).WithoutRelations, method set of *Attempt
github.com/arandu-io/hesape/database/model.(*Model[github.com/tayi-ai/arandu-attempt.Attempt]).WithoutTimestamps, method set of *Attempt
```

</details>
