import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { passwordLevelLabel, scorePassword } from '@corelog/core';

/** Four-segment strength meter. The text is announced politely; colors come from tokens via data-level. */
@Component({
  selector: 'app-password-strength',
  templateUrl: './password-strength.html',
  styleUrl: './password-strength.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PasswordStrength {
  readonly password = input('');

  protected readonly segments = [1, 2, 3, 4];
  protected readonly level = computed(() => scorePassword(this.password()));
  protected readonly message = computed(() => {
    const label = passwordLevelLabel(this.level());
    return label ? `Seguridad de la contraseña: ${label}` : '';
  });
}
