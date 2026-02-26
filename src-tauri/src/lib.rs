use serde::Serialize;

#[derive(Serialize)]
struct DesktopRuntimeInfo {
    platform: &'static str,
    app: &'static str,
}

#[tauri::command]
fn desktop_runtime_info() -> DesktopRuntimeInfo {
    DesktopRuntimeInfo {
        platform: std::env::consts::OS,
        app: "agenTerm",
    }
}

pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![desktop_runtime_info])
        .run(tauri::generate_context!())
        .expect("error while running agenTerm desktop shell");
}
