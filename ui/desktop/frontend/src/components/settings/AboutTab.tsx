import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { wails } from '../../lib/wails'

export function AboutTab() {
  const { t } = useTranslation('desktop')
  const [version, setVersion] = useState('...')

  useEffect(() => {
    wails.getVersion().then(setVersion).catch(() => setVersion('unknown'))
  }, [])

  return (
    <div className="space-y-6 max-w-lg">
      <div className="flex items-center gap-4">
        <img src="/goclaw-icon.svg" alt="GoClaw" className="h-12 w-12" />
        <div>
          <h3 className="text-base font-semibold text-text-primary">{t('about.title')}</h3>
          <p className="text-xs text-text-muted">{t('about.subtitle')}</p>
        </div>
      </div>

      <div className="rounded-lg border border-border p-4 space-y-3">
        <div className="flex justify-between text-xs">
          <span className="text-text-muted">{t('about.version')}</span>
          <span className="text-text-primary font-mono">{version}</span>
        </div>
        <div className="flex justify-between text-xs">
          <span className="text-text-muted">{t('about.edition')}</span>
          <span className="text-accent font-medium">Lite (SQLite)</span>
        </div>
        <div className="flex justify-between text-xs">
          <span className="text-text-muted">{t('about.runtime')}</span>
          <span className="text-text-primary font-mono">{t('about.runtimeValue')}</span>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-semibold text-text-primary mb-2">{t('about.editionLimits')}</h3>
        <div className="rounded-lg border border-border divide-y divide-border text-xs">
          {[
            { label: t('about.agents'), limit: t('about.maxAgents') },
            { label: t('about.teams'), limit: t('about.maxTeams') },
            { label: t('about.teamMembers'), limit: t('about.maxTeamMembers') },
            { label: t('about.database'), limit: t('about.databaseValue') },
            { label: t('about.users'), limit: t('about.usersValue') },
          ].map((item) => (
            <div key={item.label} className="flex justify-between px-3 py-2">
              <span className="text-text-muted">{item.label}</span>
              <span className="text-text-secondary">{item.limit}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="text-xs text-text-muted">
        <button
          onClick={() => BrowserOpenURL('https://github.com/nextlevelbuilder/goclaw')}
          className="text-accent hover:underline cursor-pointer"
        >
          GitHub
        </button>
        <span className="mx-2">·</span>
        <span>{t('about.builtWith')}</span>
      </div>
    </div>
  )
}
