/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type Role = 'user' | 'assistant' | 'tool' | 'system' | 'error'

export const ROLE_STYLE: Record<Role, string> = {
  user: 'bg-(--vf-role-user-bg) border-(--vf-role-user-border)',
  assistant: 'bg-(--vf-role-assistant-bg) border-(--vf-role-assistant-border)',
  tool: 'bg-(--vf-role-tool-bg) border-(--vf-role-tool-border)',
  system: 'bg-(--vf-role-system-bg) border-(--vf-role-system-border)',
  error: 'bg-(--vf-role-error-bg) border-(--vf-role-error-border)',
}

export const ROLE_DOT: Record<Role, string> = {
  user: 'bg-(--vf-role-user-dot)',
  assistant: 'bg-(--vf-role-assistant-dot)',
  tool: 'bg-(--vf-role-tool-dot)',
  system: 'bg-(--vf-role-system-dot)',
  error: 'bg-(--vf-role-error-dot)',
}

/** The visual role of a message: error wins; developer, empty and unknown roles read as system. */
export function visualRole(role: string, hasError: boolean): Role {
  if (hasError) return 'error'
  return role === 'user' || role === 'assistant' || role === 'tool'
    ? role
    : 'system'
}

export function roleStyle(role: string, hasError: boolean): string {
  return ROLE_STYLE[visualRole(role, hasError)]
}
