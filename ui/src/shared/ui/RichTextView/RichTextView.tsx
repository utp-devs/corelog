import { useMemo, useState, type ComponentPropsWithoutRef } from 'react'
import { Modal } from 'antd'
import Markdown, { type Components, type ExtraProps } from 'react-markdown'
import rehypeSanitize from 'rehype-sanitize'
import remarkGfm from 'remark-gfm'

import {
  isExternalHttpLink,
  remarkCorelog,
  richTextSchema,
  safeIssueHref,
  safeLinkHref,
  type IssueResolution,
} from '@/shared/lib/markdown'
import { IssueKey } from '../IssueKey'
import { UserChip } from '../UserChip'
import styles from './RichTextView.module.css'

const REHYPE_PLUGINS: [typeof rehypeSanitize, typeof richTextSchema][] = [[rehypeSanitize, richTextSchema]]

export type RichTextViewProps = {
  /** Markdown source (GFM). Never HTML: raw HTML is not rendered. */
  value: string
  /** Resolves an issue key to a link; undefined keeps the key as plain text. */
  resolveIssue?: (key: string) => IssueResolution | undefined
  /** Called with the user id when a mention chip is clicked. Without it, chips are static. */
  onMentionClick?: (userId: string) => void
  className?: string
}

type SpanProps = ComponentPropsWithoutRef<'span'> & ExtraProps
type AnchorProps = ComponentPropsWithoutRef<'a'> & ExtraProps
type ImageProps = ComponentPropsWithoutRef<'img'> & ExtraProps

function stringProperty(props: SpanProps, key: string): string | undefined {
  const value = props.node?.properties[key]
  return typeof value === 'string' ? value : undefined
}

/** Image shown inline; click opens it in a simple lightbox. */
function MarkdownImage(props: ImageProps) {
  const [open, setOpen] = useState(false)
  const src = props.src
  if (!src) return null
  const alt = props.alt ?? ''
  return (
    <>
      <button type="button" className={styles.imageButton} onClick={() => setOpen(true)} aria-label={`Ver imagen${alt ? `: ${alt}` : ''}`}>
        <img src={src} alt={alt} className={styles.image} />
      </button>
      <Modal open={open} footer={null} onCancel={() => setOpen(false)} title={alt || 'Imagen'} destroyOnHidden>
        <img src={src} alt={alt} className={styles.lightboxImage} />
      </Modal>
    </>
  )
}

function buildComponents(onMentionClick: ((userId: string) => void) | undefined): Components {
  return {
    a(props: AnchorProps) {
      const href = safeLinkHref(props.href)
      if (!href) {
        return <span className={styles.unsafeLink}>{props.children}</span>
      }
      if (isExternalHttpLink(href)) {
        return (
          <a href={href} target="_blank" rel="noopener noreferrer" className={styles.link}>
            {props.children}
          </a>
        )
      }
      return (
        <a href={href} className={styles.link}>
          {props.children}
        </a>
      )
    },
    img: MarkdownImage,
    span(props: SpanProps) {
      const userId = stringProperty(props, 'dataUserId')
      const userName = stringProperty(props, 'dataUserName')
      if (userId && userName) {
        const chip = <UserChip user={{ id: userId, name: userName }} size={24} />
        if (onMentionClick) {
          return (
            <button type="button" className={styles.mentionButton} onClick={() => onMentionClick(userId)}>
              {chip}
            </button>
          )
        }
        return (
          <span className={styles.mention} data-testid="rich-text-mention">
            {chip}
          </span>
        )
      }

      const issueKey = stringProperty(props, 'dataIssueKey')
      const issueHref = safeIssueHref(stringProperty(props, 'dataIssueHref'))
      if (issueKey && issueHref) {
        return (
          <IssueKey
            issueKey={issueKey}
            href={issueHref}
            summary={stringProperty(props, 'dataIssueSummary')}
            resolved={stringProperty(props, 'dataIssueResolved') === 'true'}
          />
        )
      }
      return <span>{props.children}</span>
    },
  }
}

/** Sanitized Markdown renderer: mentions as chips, issue keys as links, safe links and images. */
export function RichTextView({ value, resolveIssue, onMentionClick, className }: RichTextViewProps) {
  const remarkPlugins = useMemo(() => [remarkGfm, remarkCorelog(resolveIssue)], [resolveIssue])
  const rehypePlugins = REHYPE_PLUGINS
  const components = useMemo(() => buildComponents(onMentionClick), [onMentionClick])

  return (
    <div className={className ? `${styles.root} ${className}` : styles.root} data-testid="rich-text-view">
      <Markdown remarkPlugins={remarkPlugins} rehypePlugins={rehypePlugins} components={components}>
        {value}
      </Markdown>
    </div>
  )
}
