import { EyeInvisibleOutlined, EyeOutlined } from '@ant-design/icons'
import { Button, Input, type InputRef } from 'antd'
import type { ComponentProps, Ref } from 'react'
import { useState } from 'react'

import styles from './AuthPage.module.css'

type PasswordInputProps = Omit<ComponentProps<typeof Input>, 'type' | 'suffix'> & {
  ref?: Ref<InputRef>
}

/** Password field with a keyboard-reachable visibility toggle (aria-pressed reflects the state). */
export function PasswordInput({ ref, ...props }: PasswordInputProps) {
  const [visible, setVisible] = useState(false)

  return (
    <Input
      {...props}
      ref={ref}
      size="large"
      type={visible ? 'text' : 'password'}
      suffix={
        <Button
          type="text"
          className={styles.toggle}
          aria-label="Mostrar contraseña"
          aria-pressed={visible}
          aria-controls={props.id}
          disabled={props.disabled}
          onClick={() => setVisible((current) => !current)}
          icon={visible ? <EyeInvisibleOutlined /> : <EyeOutlined />}
        />
      }
    />
  )
}
