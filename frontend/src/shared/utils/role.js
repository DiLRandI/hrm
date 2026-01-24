import { ROLE_EMPLOYEE } from '../constants/roles.js';

export function getRole(user) {
  return user?.role || user?.RoleName || ROLE_EMPLOYEE;
}

export function isRole(user, role) {
  return getRole(user) === role;
}
