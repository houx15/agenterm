export default function SettingsModal(props: { open: boolean; onClose: () => void }) {
  if (!props.open) return null
  return (
    <div className="modal-backdrop" onClick={props.onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Settings</h3>
        </div>
        <p>Settings will appear here</p>
      </div>
    </div>
  )
}
