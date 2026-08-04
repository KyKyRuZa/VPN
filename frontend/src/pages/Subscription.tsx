import { useEffect, useState } from "react";
import { Button } from "../components/Button";
import { useAuth } from "../hooks/useAuth";
import { getSubscription, type Subscription as Sub } from "../api/subscription";
import styles from "./Subscription.module.css";

export default function Subscription() {
  const { user } = useAuth();
  const [sub, setSub] = useState<Sub | null>(null);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState<"key" | "link" | null>(null);

  useEffect(() => {
    getSubscription()
      .then(setSub)
      .catch(() => setError("Не удалось загрузить подписку"));
  }, [user?.id]);

  const copy = async (text: string, which: "key" | "link") => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(which);
      setTimeout(() => setCopied(null), 1500);
    } catch {
      setError("Не удалось скопировать в буфер обмена");
    }
  };

  return (
    <div className="section" style={{ padding: 0 }}>
      {error && <div className={styles.error}>{error}</div>}

      <div className="card">
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Ваша подписка</div>
        {sub ? (
          <div className={styles.keyBox}>{sub.subscription_url}</div>
        ) : (
          <div className={styles.keyBox}>Загрузка…</div>
        )}
        <div className={styles.actions}>
          <Button disabled={!sub} onClick={() => sub && copy(sub.subscription_url, "link")}>
            {copied === "link" ? "Скопировано" : "Скопировать ссылку"}
          </Button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div style={{ fontWeight: 700, marginBottom: 10 }}>Имя пользователя</div>
        <div className={styles.keyBox}>{sub?.username ?? "—"}</div>
        <div className={styles.actions}>
          <Button
            variant="secondary"
            disabled={!sub}
            onClick={() => sub && copy(sub.username, "key")}
          >
            {copied === "key" ? "Скопировано" : "Скопировать"}
          </Button>
        </div>
      </div>
    </div>
  );
}
