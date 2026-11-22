import { Button, Result } from 'antd'
import { useNavigate } from 'react-router-dom'

import styles from './NotFoundPage.module.css'

export function NotFoundPage() {
  const navigate = useNavigate()

  return (
    <div className={styles.wrapper}>
      <Result
        status="404"
        title="404"
        subTitle="Lo sentimos, la página que buscas no existe."
        extra={
            <Button type="primary" onClick={() => navigate('/issues')}>
            Volver a incidencias
          </Button>
        }
      />
    </div>
  )
}
