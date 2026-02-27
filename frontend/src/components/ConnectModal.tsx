export default function ConnectModal(props: { open: boolean; onClose: () => void }) {
  if (!props.open) return null
  return (
    <div className="modal-backdrop" onClick={props.onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Connect Mobile</h3>
        </div>
        <p>QR code will appear here</p>
      </div>
    </div>
  )
}
