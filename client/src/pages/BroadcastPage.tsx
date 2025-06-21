import React, { useState } from "react";
import { Button } from "../components/ui/button";
import { useNavigate } from "react-router-dom";

// TODO: Replace with real admin check
function useIsAdmin() {
  // This should check actual user context or role
  // For now, returns true for demonstration
  return true;
}

const BroadcastPage: React.FC = () => {
  const [message, setMessage] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const isAdmin = useIsAdmin();
  const navigate = useNavigate();

  if (!isAdmin) {
    return (
      <div className="flex flex-col items-center justify-center min-h-screen">
        <h1 className="text-2xl font-bold text-red-600 mb-4">Access Denied</h1>
        <p className="mb-6">You must be an admin to access this page.</p>
        <Button onClick={() => navigate("/")}>Go Home</Button>
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setStatus(null);
    try {
      const res = await fetch("/api/broadcast", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ message }),
        credentials: "include",
      });
      if (res.ok) {
        setStatus("Broadcast sent successfully!");
        setMessage("");
      } else {
        const err = await res.text();
        setStatus(`Failed: ${err}`);
      }
    } catch (error: any) {
      setStatus(error.message || "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gradient-to-br from-blue-50 to-blue-200 py-12 px-4">
      <div className="bg-white shadow-lg rounded-xl p-8 w-full max-w-md">
        <h1 className="text-3xl font-bold text-blue-700 mb-6 text-center">Broadcast Message</h1>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <textarea
            className="border rounded-lg p-3 min-h-[100px] text-lg"
            value={message}
            onChange={e => setMessage(e.target.value)}
            placeholder="Enter your broadcast message..."
            required
            disabled={loading}
          />
          <Button type="submit" disabled={loading || !message.trim()}>
            {loading ? "Sending..." : "Send Broadcast"}
          </Button>
        </form>
        {status && <div className="mt-4 text-center text-sm text-blue-600">{status}</div>}
      </div>
    </div>
  );
};

export default BroadcastPage;
