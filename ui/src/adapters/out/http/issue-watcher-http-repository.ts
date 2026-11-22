import { apiClient } from './api-client'
import type { IssueWatcherState } from '@/core/tickets/domain/issue-watcher'

export class IssueWatcherHttpRepository {
  async getState(issueId: string): Promise<IssueWatcherState> {
    const { data } = await apiClient.get<{ data: IssueWatcherState }>(`/issues/${issueId}/watchers`)
    return data.data
  }

  async watch(issueId: string): Promise<IssueWatcherState> {
    const { data } = await apiClient.post<{ data: IssueWatcherState }>(`/issues/${issueId}/watchers`)
    return data.data
  }

  async unwatch(issueId: string): Promise<IssueWatcherState> {
    const { data } = await apiClient.delete<{ data: IssueWatcherState }>(`/issues/${issueId}/watchers`)
    return data.data
  }
}
