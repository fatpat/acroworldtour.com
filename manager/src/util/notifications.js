import { useSnackbar } from 'notistack';

export const useNotifications = (props) => {
  const { enqueueSnackbar, closeSnackbar } = useSnackbar();
  const notification = (text, type) => {
    const k = enqueueSnackbar(text, {
      variant: type,
      onClick: () => closeSnackbar(k)
    })
  }
  const success = (text) => {notification(text, 'success')}
  const error = (text) => {notification(text, 'error')}
  const warning = (text) => {notification(text, 'warning')}
  const info = (text) => {notification(text, 'info')}

  return [success, info, warning, error]
}
