import { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

export default function VerifyEmail() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [code, setCode] = useState(['', '', '', '', '', '']);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [resendCooldown, setResendCooldown] = useState(0);
  const [successMessage, setSuccessMessage] = useState('');
  const [initialSendDone, setInitialSendDone] = useState(false);
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

  // Redirect if already verified
  useEffect(() => {
    if (user?.email_verified) {
      navigate('/');
    }
  }, [user, navigate]);

  // Auto-send verification code on page load (only once)
  useEffect(() => {
    const sendInitialCode = async () => {
      if (initialSendDone || !user || user.email_verified) return;

      setInitialSendDone(true);
      setLoading(true);

      try {
        const response = await fetch('/api/auth/resend-verification', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('access_token')}`,
          },
        });

        const data = await response.json();

        if (response.ok) {
          setSuccessMessage('Verification code sent to your email!');
          setResendCooldown(60);
        } else if (data.error !== 'ALREADY_VERIFIED') {
          // Only show error if it's not "already verified"
          console.log('Initial verification send:', data.message || 'Code sent');
        }
      } catch (err) {
        console.log('Initial verification send failed, user can click Resend');
      } finally {
        setLoading(false);
      }
    };

    sendInitialCode();
  }, [user, initialSendDone]);

  // Handle resend cooldown timer
  useEffect(() => {
    if (resendCooldown > 0) {
      const timer = setTimeout(() => {
        setResendCooldown(resendCooldown - 1);
      }, 1000);
      return () => clearTimeout(timer);
    }
  }, [resendCooldown]);

  const handleChange = (index: number, value: string) => {
    // Only allow single digit
    const digit = value.replace(/[^0-9]/g, '').slice(0, 1);

    const newCode = [...code];
    newCode[index] = digit;
    setCode(newCode);
    setError('');

    // Auto-focus next input
    if (digit && index < 5) {
      inputRefs.current[index + 1]?.focus();
    }

    // Auto-submit when all 6 digits are entered
    if (index === 5 && digit) {
      const fullCode = newCode.join('');
      if (fullCode.length === 6) {
        handleSubmit(fullCode);
      }
    }
  };

  const handleKeyDown = (index: number, e: React.KeyboardEvent<HTMLInputElement>) => {
    // Handle backspace
    if (e.key === 'Backspace') {
      if (!code[index] && index > 0) {
        inputRefs.current[index - 1]?.focus();
      }
    }
    // Handle left arrow
    else if (e.key === 'ArrowLeft' && index > 0) {
      inputRefs.current[index - 1]?.focus();
    }
    // Handle right arrow
    else if (e.key === 'ArrowRight' && index < 5) {
      inputRefs.current[index + 1]?.focus();
    }
  };

  const handlePaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    const pastedData = e.clipboardData.getData('text').replace(/[^0-9]/g, '').slice(0, 6);
    const newCode = [...code];

    for (let i = 0; i < pastedData.length; i++) {
      newCode[i] = pastedData[i];
    }

    setCode(newCode);
    setError('');

    // Focus the next empty input or the last input
    const nextIndex = Math.min(pastedData.length, 5);
    inputRefs.current[nextIndex]?.focus();

    // Auto-submit if 6 digits pasted
    if (pastedData.length === 6) {
      handleSubmit(pastedData);
    }
  };

  const handleSubmit = async (codeValue?: string) => {
    const verificationCode = codeValue || code.join('');

    if (verificationCode.length !== 6) {
      setError('Please enter all 6 digits');
      return;
    }

    setLoading(true);
    setError('');

    try {
      const response = await fetch('/api/auth/verify-email', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('access_token')}`,
        },
        body: JSON.stringify({ code: verificationCode }),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.message || 'Verification failed');
      }

      setSuccessMessage('Email verified successfully! Redirecting...');
      setTimeout(() => {
        navigate('/');
        window.location.reload(); // Reload to update user context
      }, 1500);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed');
      setCode(['', '', '', '', '', '']);
      inputRefs.current[0]?.focus();
    } finally {
      setLoading(false);
    }
  };

  const handleResend = async () => {
    if (resendCooldown > 0) return;

    setLoading(true);
    setError('');
    setSuccessMessage('');

    try {
      const response = await fetch('/api/auth/resend-verification', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('access_token')}`,
        },
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.message || 'Failed to resend code');
      }

      setSuccessMessage('Verification code sent to your email!');
      setResendCooldown(60);
      setCode(['', '', '', '', '', '']);
      inputRefs.current[0]?.focus();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to resend code');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-dark-900 flex items-center justify-center px-4">
      <div className="max-w-md w-full">
        <div className="bg-dark-800 rounded-lg border border-dark-700 p-8">
          <div className="text-center mb-8">
            <div className="mx-auto w-16 h-16 bg-indigo-600 rounded-full flex items-center justify-center mb-4">
              <svg className="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
              </svg>
            </div>
            <h2 className="text-2xl font-bold text-white mb-2">Verify Your Email</h2>
            <p className="text-gray-400">
              We've sent a 6-digit verification code to<br />
              <span className="text-indigo-400 font-medium">{user?.email}</span>
            </p>
          </div>

          <div className="space-y-6">
            {/* 6-digit code input */}
            <div className="flex justify-center gap-2">
              {code.map((digit, index) => (
                <input
                  key={index}
                  ref={(el) => (inputRefs.current[index] = el)}
                  type="text"
                  inputMode="numeric"
                  maxLength={1}
                  value={digit}
                  onChange={(e) => handleChange(index, e.target.value)}
                  onKeyDown={(e) => handleKeyDown(index, e)}
                  onPaste={index === 0 ? handlePaste : undefined}
                  className="w-12 h-14 text-center text-2xl font-bold bg-dark-700 border-2 border-dark-600 rounded-lg text-white focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none transition-colors"
                  disabled={loading}
                  autoFocus={index === 0}
                />
              ))}
            </div>

            {/* Error message */}
            {error && (
              <div className="bg-red-900/20 border border-red-500 text-red-500 px-4 py-3 rounded-lg text-sm text-center">
                {error}
              </div>
            )}

            {/* Success message */}
            {successMessage && (
              <div className="bg-green-900/20 border border-green-500 text-green-500 px-4 py-3 rounded-lg text-sm text-center">
                {successMessage}
              </div>
            )}

            {/* Resend button */}
            <div className="text-center">
              <p className="text-gray-400 text-sm mb-2">
                Didn't receive the code?
              </p>
              <button
                onClick={handleResend}
                disabled={loading || resendCooldown > 0}
                className={`text-sm font-medium ${
                  resendCooldown > 0
                    ? 'text-gray-500 cursor-not-allowed'
                    : 'text-indigo-400 hover:text-indigo-300'
                } transition-colors`}
              >
                {resendCooldown > 0 ? `Resend in ${resendCooldown}s` : 'Resend Code'}
              </button>
            </div>

            {/* Manual submit button (optional) */}
            <button
              onClick={() => handleSubmit()}
              disabled={loading || code.join('').length !== 6}
              className="w-full bg-indigo-600 hover:bg-indigo-700 disabled:bg-gray-700 disabled:cursor-not-allowed text-white font-medium py-3 px-4 rounded-lg transition-colors"
            >
              {loading ? 'Verifying...' : 'Verify Email'}
            </button>
          </div>

          <div className="mt-6 text-center">
            <p className="text-gray-500 text-xs">
              The code will expire in 15 minutes
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
