import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
// @ts-expect-error helper will be created after the failing test is observed.
import { createDeleteTaskConfirmation } from './deleteTaskConfirmation.ts'

describe('createDeleteTaskConfirmation', () => {
  it('requires a separate confirm step before deletion runs', async () => {
    const deletedTaskIds: number[] = []
    const confirmation = createDeleteTaskConfirmation(async (task) => {
      deletedTaskIds.push(task.id)
    })

    const task = {
      id: 42,
      name: '日报任务',
    }

    confirmation.request(task)

    assert.equal(confirmation.pendingTask()?.id, 42)
    assert.deepEqual(deletedTaskIds, [])

    await confirmation.confirm()

    assert.deepEqual(deletedTaskIds, [42])
    assert.equal(confirmation.pendingTask(), null)
  })

  it('clears the pending task when the user cancels', () => {
    const confirmation = createDeleteTaskConfirmation(async () => {})

    confirmation.request({
      id: 7,
      name: '周报任务',
    })
    confirmation.cancel()

    assert.equal(confirmation.pendingTask(), null)
  })
})
