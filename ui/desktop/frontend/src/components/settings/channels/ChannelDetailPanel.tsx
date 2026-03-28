import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useChannelDetail } from '../../../hooks/use-channel-detail'
import { useAgentCrud } from '../../../hooks/use-agent-crud'
import { ChannelGeneralTab } from './ChannelGeneralTab'
import { ChannelCredentialsTab } from './ChannelCredentialsTab'
import { ChannelManagersTab } from './ChannelManagersTab'
import { ChannelAdvancedTab } from './ChannelAdvancedTab'
import type { ChannelStatus } from '../../../types/channel'

interface ChannelDetailPanelProps {
  instanceId: string
  status: ChannelStatus | null
  onBack: () => void
  onDelete: () => void
}

const TABS = ['general', 'credentials', 'managers', 'advanced'] as const
type TabKey = (typeof TABS)[number]

export function ChannelDetailPanel({ instanceId, status, onBack, onDelete }: ChannelDetailPanelProps) {
  const { t } = useTranslation('channels')
  const { instance, loading, updateInstance, listManagerGroups, listManagers, addManager, removeManager, listContacts } = useChannelDetail(instanceId)
  const { agents } = useAgentCrud()
  const [activeTab, setActiveTab] = useState<TabKey>('general')

  if (loading || !instance) {
    return (
      <div className="space-y-3">
        <button onClick={onBack} className="text-xs text-text-muted hover:text-text-primary transition-colors cursor-pointer">
          ← {t('detail.back')}
        </button>
        <div className="space-y-2">
          {[1, 2, 3].map((i) => <div key={i} className="h-10 rounded-lg bg-surface-tertiary/50 animate-pulse" />)}
        </div>
      </div>
    )
  }

  // Status
  let dotColor = 'bg-gray-400'
  let statusText = t('status.disabled')
  if (instance.enabled && status?.running) { dotColor = 'bg-emerald-500'; statusText = t('status.running') }
  else if (instance.enabled) { dotColor = 'bg-amber-500'; statusText = t('status.stopped') }

  const agentName = (() => {
    const a = agents.find((a) => a.id === instance.agent_id)
    return a?.display_name || a?.agent_key || instance.agent_id.slice(0, 8)
  })()

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3 min-w-0">
          <button onClick={onBack} className="text-xs text-text-muted hover:text-text-primary transition-colors shrink-0 cursor-pointer">
            ← {t('detail.back')}
          </button>
          <span className="text-sm font-medium text-text-primary truncate">{instance.display_name || instance.name}</span>
          <span className="rounded-full px-1.5 py-0.5 text-[10px] bg-surface-tertiary text-text-secondary border border-border shrink-0">
            {t(`channelTypes.${instance.channel_type}`)}
          </span>
          <span className={`w-1.5 h-1.5 rounded-full ${dotColor} shrink-0`} />
          <span className="text-[11px] text-text-muted shrink-0">{statusText}</span>
          <span className="text-[11px] text-text-muted shrink-0">· {agentName}</span>
        </div>
        <button onClick={onDelete} className="px-2.5 py-1 text-[11px] border border-border rounded-lg text-error hover:bg-error/10 transition-colors cursor-pointer shrink-0">
          {t('delete.confirmLabel')}
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border">
        {TABS.map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-3 py-1.5 text-xs transition-colors cursor-pointer ${
              activeTab === tab
                ? 'text-accent font-medium border-b-2 border-accent -mb-px'
                : 'text-text-muted hover:text-text-primary'
            }`}
          >
            {t(`detail.tabs.${tab}`)}
          </button>
        ))}
      </div>

      {/* Tab content */}
      <div>
        {activeTab === 'general' && <ChannelGeneralTab instance={instance} agents={agents} onUpdate={updateInstance} />}
        {activeTab === 'credentials' && <ChannelCredentialsTab instance={instance} onUpdate={updateInstance} />}
        {activeTab === 'managers' && (
          <ChannelManagersTab
            listManagerGroups={listManagerGroups}
            listManagers={listManagers}
            addManager={addManager}
            removeManager={removeManager}
            listContacts={listContacts}
          />
        )}
        {activeTab === 'advanced' && <ChannelAdvancedTab instance={instance} onUpdate={updateInstance} />}
      </div>
    </div>
  )
}
