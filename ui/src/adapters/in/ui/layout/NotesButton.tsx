import { EditOutlined } from '@ant-design/icons'
import { Button, Tooltip } from 'antd'

/** Notes arrive in P8; the button is visible but disabled so the header layout is final. */
export function NotesButton() {
  return (
    <Tooltip title="Próximamente">
      <span className={undefined}>
        <Button type="text" icon={<EditOutlined />} disabled aria-label="Notas (próximamente)" />
      </span>
    </Tooltip>
  )
}
