use serde::Serialize;
use std::fs::{self, OpenOptions};
use std::net::{SocketAddr, TcpStream};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::{Mutex, OnceLock};
use std::thread;
use std::time::Duration;

const BACKEND_HOST: &str = "127.0.0.1";
const BACKEND_PORT: u16 = 8765;
const BACKEND_TOKEN: &str = "agenterm-desktop-local";
const BACKEND_DB_PATH: &str = ".cache/desktop/agenterm.db";
const BACKEND_AGENTS_DIR: &str = "configs/agents";
const BACKEND_PLAYBOOKS_DIR: &str = "configs/playbooks";

static BACKEND_CHILD: OnceLock<Mutex<Option<Child>>> = OnceLock::new();

#[derive(Serialize)]
struct DesktopRuntimeInfo {
    platform: &'static str,
    app: &'static str,
    backend_url: String,
    backend_token: &'static str,
    sidecar_managed: bool,
}

fn project_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("..")
}

fn backend_addr() -> SocketAddr {
    format!("{}:{}", BACKEND_HOST, BACKEND_PORT)
        .parse()
        .unwrap_or_else(|_| SocketAddr::from(([127, 0, 0, 1], BACKEND_PORT)))
}

fn backend_alive() -> bool {
    TcpStream::connect_timeout(&backend_addr(), Duration::from_millis(300)).is_ok()
}

fn sidecar_disabled() -> bool {
    matches!(std::env::var("AGENTERM_NO_SIDECAR"), Ok(v) if v == "1" || v.eq_ignore_ascii_case("true"))
}

fn spawn_backend_sidecar() -> Result<bool, String> {
    if sidecar_disabled() || backend_alive() {
        return Ok(false);
    }
    let root = project_root();
    let cache_dir = root.join(".cache").join("desktop");
    fs::create_dir_all(&cache_dir)
        .map_err(|e| format!("create desktop cache directory {}: {}", cache_dir.display(), e))?;

    let lock = BACKEND_CHILD.get_or_init(|| Mutex::new(None));
    let mut guard = lock
        .lock()
        .map_err(|_| String::from("backend sidecar lock poisoned"))?;
    if guard.is_some() {
        return Ok(false);
    }

    let mut cmd = if cfg!(debug_assertions) {
        let launch = format!(
            "source ~/.zshrc >/dev/null 2>&1; exec go run ./cmd/agenterm --port {} --token {} --db-path {} --agents-dir {} --playbooks-dir {} --dir .",
            BACKEND_PORT,
            BACKEND_TOKEN,
            BACKEND_DB_PATH,
            BACKEND_AGENTS_DIR,
            BACKEND_PLAYBOOKS_DIR
        );
        let mut c = Command::new("zsh");
        c.arg("-lc").arg(launch);
        c
    } else {
        let exe = std::env::current_exe().map_err(|e| format!("resolve current exe: {}", e))?;
        let exe_dir = exe
            .parent()
            .ok_or_else(|| String::from("resolve executable directory"))?;
        let candidates = [exe_dir.join("agenterm"), exe_dir.join("agenterm-server")];
        let binary = candidates
            .iter()
            .find(|candidate| candidate.exists())
            .ok_or_else(|| String::from("desktop backend binary not found near app executable"))?;
        Command::new(binary)
    };

    cmd.current_dir(&root);
    if !cfg!(debug_assertions) {
        cmd.arg("--port")
            .arg(BACKEND_PORT.to_string())
            .arg("--token")
            .arg(BACKEND_TOKEN)
            .arg("--db-path")
            .arg(BACKEND_DB_PATH)
            .arg("--agents-dir")
            .arg(BACKEND_AGENTS_DIR)
            .arg("--playbooks-dir")
            .arg(BACKEND_PLAYBOOKS_DIR)
            .arg("--dir")
            .arg(".");
    }

    if cfg!(debug_assertions) {
        let log_path = cache_dir.join("backend-sidecar.log");
        let log_file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&log_path)
            .map_err(|e| format!("open sidecar log {}: {}", log_path.display(), e))?;
        let log_file_err = log_file
            .try_clone()
            .map_err(|e| format!("clone sidecar log handle {}: {}", log_path.display(), e))?;
        cmd.stdout(Stdio::from(log_file))
            .stderr(Stdio::from(log_file_err));
    } else {
        cmd.stdout(Stdio::null()).stderr(Stdio::null());
    }

    let mut child = cmd
        .spawn()
        .map_err(|e| format!("spawn backend sidecar failed: {}", e))?;

    thread::sleep(Duration::from_millis(600));
    if let Ok(Some(status)) = child.try_wait() {
        return Err(format!(
            "backend sidecar exited early with status {}",
            status
        ));
    }

    *guard = Some(child);
    Ok(true)
}

fn stop_backend_sidecar() {
    if let Some(lock) = BACKEND_CHILD.get() {
        if let Ok(mut guard) = lock.lock() {
            if let Some(mut child) = guard.take() {
                let _ = child.kill();
                let _ = child.wait();
            }
        }
    }
}

#[tauri::command]
fn desktop_runtime_info() -> DesktopRuntimeInfo {
    let sidecar_managed = BACKEND_CHILD
        .get()
        .and_then(|lock| lock.lock().ok().map(|g| g.is_some()))
        .unwrap_or(false);
    DesktopRuntimeInfo {
        platform: std::env::consts::OS,
        app: "agenTerm",
        backend_url: format!("http://{}:{}", BACKEND_HOST, BACKEND_PORT),
        backend_token: BACKEND_TOKEN,
        sidecar_managed,
    }
}

pub fn run() {
    let context = tauri::generate_context!();
    let builder = tauri::Builder::default()
        .setup(|_| {
            if let Err(err) = spawn_backend_sidecar() {
                eprintln!("[agenTerm] backend sidecar startup warning: {err}");
            }
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![desktop_runtime_info]);

    let app = builder
        .build(context)
        .expect("error while running agenTerm desktop shell");

    app.run(|_, event| {
        if matches!(
            event,
            tauri::RunEvent::Exit | tauri::RunEvent::ExitRequested { .. }
        ) {
            stop_backend_sidecar();
        }
    });
}
