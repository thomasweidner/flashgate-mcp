Set-StrictMode -Version 3.0

if (-not ('FlashGate.Validation.BoundedProcessRunner' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Globalization;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;

namespace FlashGate.Validation
{
    public sealed class BoundedProcessResult
    {
        public bool Attempted { get; set; }
        public string Status { get; set; } = "FAIL";
        public int? ExitCode { get; set; }
        public bool TimedOut { get; set; }
        public bool OutputLimitExceeded { get; set; }
        public string Stdout { get; set; } = "";
        public string Stderr { get; set; } = "";
        public string FailureReason { get; set; } = "";
    }

    internal sealed class ReadyBarrierTestResult
    {
        public BoundedProcessResult ProcessResult { get; set; } =
            new BoundedProcessResult();
        public bool ReadyObserved { get; set; }
        public int? ReadyProcessId { get; set; }
        public bool ReleasedObserved { get; set; }
        public int? ReleasedProcessId { get; set; }
        public bool ActivatedObserved { get; set; }
        public int? ActivatedProcessId { get; set; }
        public bool ChildAliveBeforeTimeout { get; set; }
        public bool TimeoutStartedAfterReady { get; set; }
        public long ReadyElapsedMilliseconds { get; set; }
        public string BarrierFailureReason { get; set; } = "";
    }

    public static class BoundedProcessRunner
    {
        [StructLayout(LayoutKind.Sequential)]
        private struct ProcessBasicInformation
        {
            public IntPtr Reserved1;
            public IntPtr PebBaseAddress;
            public IntPtr Reserved2_0;
            public IntPtr Reserved2_1;
            public IntPtr UniqueProcessId;
            public IntPtr InheritedFromUniqueProcessId;
        }

        private sealed class ControlledProcessIdentity
        {
            public int ProcessId;
            public long StartTimeUtcTicks;
        }

        [DllImport("ntdll.dll")]
        private static extern int NtQueryInformationProcess(
            IntPtr processHandle,
            int processInformationClass,
            ref ProcessBasicInformation processInformation,
            int processInformationLength,
            out int returnLength);

        private enum TerminationOutcome
        {
            NotAttempted,
            AlreadyExited,
            TreeKillSucceeded,
            FallbackKillSucceeded,
            BothFailed
        }

        private sealed class CaptureState
        {
            public readonly object Sync = new object();
            public bool OutputLimitExceeded;
            public string FailureReason = "";
            public TerminationOutcome Termination =
                TerminationOutcome.NotAttempted;
        }

        private static bool HasExited(Process process)
        {
            try
            {
                return process.HasExited;
            }
            catch
            {
                return false;
            }
        }

        private static bool TryGetLiveIdentity(
            int processId,
            out ControlledProcessIdentity identity)
        {
            identity = null;
            try
            {
                using (Process process = Process.GetProcessById(processId))
                {
                    if (process.HasExited)
                    {
                        return false;
                    }
                    long startTimeUtcTicks =
                        process.StartTime.ToUniversalTime().Ticks;
                    if (process.HasExited)
                    {
                        return false;
                    }
                    identity = new ControlledProcessIdentity
                    {
                        ProcessId = processId,
                        StartTimeUtcTicks = startTimeUtcTicks
                    };
                    return true;
                }
            }
            catch
            {
                return false;
            }
        }

        private static bool TryGetParentProcessId(
            Process process,
            out int parentProcessId)
        {
            parentProcessId = 0;
            try
            {
                var information = new ProcessBasicInformation();
                int returnLength;
                int status = NtQueryInformationProcess(
                    process.Handle,
                    0,
                    ref information,
                    Marshal.SizeOf<ProcessBasicInformation>(),
                    out returnLength);
                long parentValue =
                    information.InheritedFromUniqueProcessId.ToInt64();
                if (
                    status != 0 ||
                    parentValue <= 0 ||
                    parentValue > Int32.MaxValue)
                {
                    return false;
                }
                parentProcessId = (int)parentValue;
                return true;
            }
            catch
            {
                return false;
            }
        }

        private static bool IsDescendantOf(
            int childProcessId,
            int ancestorProcessId)
        {
            var visited = new HashSet<int>();
            int currentProcessId = childProcessId;
            for (int depth = 0; depth < 64; depth++)
            {
                if (!visited.Add(currentProcessId))
                {
                    return false;
                }
                try
                {
                    using (
                        Process current =
                            Process.GetProcessById(currentProcessId))
                    {
                        if (current.HasExited)
                        {
                            return false;
                        }
                        int parentProcessId;
                        if (
                            !TryGetParentProcessId(
                                current,
                                out parentProcessId))
                        {
                            return false;
                        }
                        if (parentProcessId == ancestorProcessId)
                        {
                            return true;
                        }
                        if (
                            parentProcessId <= 0 ||
                            parentProcessId == currentProcessId)
                        {
                            return false;
                        }
                        currentProcessId = parentProcessId;
                    }
                }
                catch
                {
                    return false;
                }
            }
            return false;
        }

        private static string ValidateControlledChild(
            Process rootProcess,
            int childProcessId,
            ControlledProcessIdentity expectedIdentity,
            ControlledProcessIdentity expectedRootIdentity,
            out ControlledProcessIdentity actualIdentity,
            out ControlledProcessIdentity actualRootIdentity)
        {
            actualIdentity = null;
            actualRootIdentity = null;
            if (HasExited(rootProcess))
            {
                return "ReadyParentNotAlive";
            }
            if (!TryGetLiveIdentity(rootProcess.Id, out actualRootIdentity))
            {
                return "ReadyParentNotAlive";
            }
            if (
                expectedRootIdentity != null &&
                (
                    actualRootIdentity.ProcessId !=
                        expectedRootIdentity.ProcessId ||
                    actualRootIdentity.StartTimeUtcTicks !=
                        expectedRootIdentity.StartTimeUtcTicks
                ))
            {
                return "ReadyParentIdentityMismatch";
            }
            if (!TryGetLiveIdentity(childProcessId, out actualIdentity))
            {
                return "ReadyChildNotAlive";
            }
            if (
                expectedIdentity != null &&
                (
                    actualIdentity.ProcessId != expectedIdentity.ProcessId ||
                    actualIdentity.StartTimeUtcTicks !=
                        expectedIdentity.StartTimeUtcTicks
                ))
            {
                return "ReadyChildIdentityMismatch";
            }
            if (!IsDescendantOf(childProcessId, rootProcess.Id))
            {
                return "ReadyChildIdentityMismatch";
            }
            return "";
        }

        private static string ReadPidRecord(
            string path,
            string recordName,
            out int processId)
        {
            processId = 0;
            byte[] bytes;
            try
            {
                using (
                    var stream = new FileStream(
                        path,
                        FileMode.Open,
                        FileAccess.Read,
                        FileShare.ReadWrite | FileShare.Delete))
                {
                    if (stream.Length > 64)
                    {
                        return recordName + "PidInvalid";
                    }
                    bytes = new byte[(int)stream.Length];
                    int offset = 0;
                    while (offset < bytes.Length)
                    {
                        int count = stream.Read(
                            bytes,
                            offset,
                            bytes.Length - offset);
                        if (count == 0)
                        {
                            break;
                        }
                        offset += count;
                    }
                    if (
                        offset != bytes.Length ||
                        stream.ReadByte() != -1)
                    {
                        return recordName + "PidInvalid";
                    }
                }
            }
            catch
            {
                return recordName + "ReadFailure";
            }

            string text;
            try
            {
                text = new UTF8Encoding(false, true).GetString(bytes);
            }
            catch
            {
                return recordName + "PidInvalid";
            }
            Match match = Regex.Match(
                text,
                @"\A" + recordName.ToUpperInvariant() +
                    @":([1-9][0-9]*)\z",
                RegexOptions.CultureInvariant);
            if (
                !match.Success ||
                !Int32.TryParse(
                    match.Groups[1].Value,
                    NumberStyles.None,
                    CultureInfo.InvariantCulture,
                    out processId) ||
                processId <= 0)
            {
                processId = 0;
                return recordName + "PidInvalid";
            }
            return "";
        }

        private static TerminationOutcome TryTerminate(Process process)
        {
            if (HasExited(process))
            {
                return TerminationOutcome.AlreadyExited;
            }
            try
            {
                process.Kill(true);
                return TerminationOutcome.TreeKillSucceeded;
            }
            catch
            {
                if (HasExited(process))
                {
                    return TerminationOutcome.AlreadyExited;
                }
                try
                {
                    process.Kill();
                    return TerminationOutcome.FallbackKillSucceeded;
                }
                catch
                {
                    return HasExited(process)
                        ? TerminationOutcome.AlreadyExited
                        : TerminationOutcome.BothFailed;
                }
            }
        }

        private static void RecordTermination(
            CaptureState state,
            TerminationOutcome outcome)
        {
            lock (state.Sync)
            {
                if (
                    outcome == TerminationOutcome.BothFailed ||
                    state.Termination == TerminationOutcome.NotAttempted)
                {
                    state.Termination = outcome;
                }
                if (
                    outcome == TerminationOutcome.BothFailed &&
                    String.IsNullOrEmpty(state.FailureReason))
                {
                    state.FailureReason = "TerminationFailed";
                }
            }
        }

        private static Task CaptureAsync(
            StreamReader reader,
            StringBuilder output,
            int maximumCharacters,
            string streamName,
            Process process,
            CaptureState state,
            Func<Process, TerminationOutcome> terminate)
        {
            return Task.Run(() =>
            {
                char[] buffer = new char[4096];
                try
                {
                    while (true)
                    {
                        int count = reader.Read(buffer, 0, buffer.Length);
                        if (count == 0)
                        {
                            return;
                        }

                        int remaining;
                        lock (output)
                        {
                            remaining = maximumCharacters - output.Length;
                            if (remaining > 0)
                            {
                                output.Append(
                                    buffer,
                                    0,
                                    Math.Min(count, remaining));
                            }
                        }
                        if (count > remaining)
                        {
                            lock (state.Sync)
                            {
                                state.OutputLimitExceeded = true;
                                if (String.IsNullOrEmpty(state.FailureReason))
                                {
                                    state.FailureReason =
                                        streamName + "LimitExceeded";
                                }
                            }
                            RecordTermination(state, terminate(process));
                            return;
                        }
                    }
                }
                catch (Exception exception)
                {
                    lock (state.Sync)
                    {
                        if (String.IsNullOrEmpty(state.FailureReason))
                        {
                            state.FailureReason =
                                streamName + "ReadFailure:" +
                            exception.GetType().Name;
                        }
                    }
                    RecordTermination(state, terminate(process));
                }
            });
        }

        public static BoundedProcessResult Run(
            string filePath,
            IReadOnlyList<string> arguments,
            string workingDirectory,
            int timeoutMilliseconds,
            int maximumCharactersPerStream,
            int cleanupTimeoutMilliseconds)
        {
            return RunCore(
                filePath,
                arguments,
                workingDirectory,
                timeoutMilliseconds,
                maximumCharactersPerStream,
                cleanupTimeoutMilliseconds,
                TryTerminate,
                null,
                null);
        }

        internal static BoundedProcessResult RunTerminationFailureTestOnly(
            string filePath,
            IReadOnlyList<string> arguments,
            string workingDirectory,
            int timeoutMilliseconds,
            int maximumCharactersPerStream,
            int cleanupTimeoutMilliseconds)
        {
            return RunCore(
                filePath,
                arguments,
                workingDirectory,
                timeoutMilliseconds,
                maximumCharactersPerStream,
                cleanupTimeoutMilliseconds,
                process => TerminationOutcome.BothFailed,
                null,
                null);
        }

        internal static ReadyBarrierTestResult RunReadyBarrierTestOnly(
            string filePath,
            IReadOnlyList<string> arguments,
            string workingDirectory,
            int timeoutMilliseconds,
            int maximumCharactersPerStream,
            int cleanupTimeoutMilliseconds,
            string readyPath,
            string releasePath,
            string releasedPath,
            string timeoutActivationProbePath,
            string timeoutActivatedPath,
            string timeoutFinalCheckPath,
            int readyTimeoutMilliseconds,
            string barrierMode)
        {
            var barrierResult = new ReadyBarrierTestResult();
            ControlledProcessIdentity childIdentity = null;
            ControlledProcessIdentity rootIdentity = null;
            Func<Process, string> beforeTimedWait = process =>
            {
                var readyClock = Stopwatch.StartNew();
                while (readyClock.ElapsedMilliseconds < readyTimeoutMilliseconds)
                {
                    if (File.Exists(readyPath))
                    {
                        int childProcessId;
                        string recordFailure = ReadPidRecord(
                            readyPath,
                            "Ready",
                            out childProcessId);
                        if (!String.IsNullOrEmpty(recordFailure))
                        {
                            barrierResult.BarrierFailureReason =
                                recordFailure;
                            return barrierResult.BarrierFailureReason;
                        }

                        barrierResult.ReadyObserved = true;
                        barrierResult.ReadyProcessId = childProcessId;
                        barrierResult.ReadyElapsedMilliseconds =
                            readyClock.ElapsedMilliseconds;
                        string identityFailure = ValidateControlledChild(
                            process,
                            childProcessId,
                            null,
                            null,
                            out childIdentity,
                            out rootIdentity);
                        if (!String.IsNullOrEmpty(identityFailure))
                        {
                            barrierResult.BarrierFailureReason =
                                identityFailure;
                            return barrierResult.BarrierFailureReason;
                        }
                        if (
                            barrierMode ==
                                "child-identity-mismatch-after-ready")
                        {
                            childIdentity.StartTimeUtcTicks++;
                        }
                        if (
                            barrierMode ==
                                "parent-identity-mismatch-after-ready")
                        {
                            rootIdentity.StartTimeUtcTicks++;
                        }

                        Thread.Sleep(75);
                        ControlledProcessIdentity revalidatedIdentity;
                        ControlledProcessIdentity revalidatedRootIdentity;
                        identityFailure = ValidateControlledChild(
                            process,
                            childProcessId,
                            childIdentity,
                            rootIdentity,
                            out revalidatedIdentity,
                            out revalidatedRootIdentity);
                        if (!String.IsNullOrEmpty(identityFailure))
                        {
                            barrierResult.BarrierFailureReason =
                                identityFailure;
                            return barrierResult.BarrierFailureReason;
                        }

                        try
                        {
                            File.WriteAllText(releasePath, "RELEASE");
                        }
                        catch
                        {
                            barrierResult.BarrierFailureReason =
                                "ReleaseWriteFailure";
                            return barrierResult.BarrierFailureReason;
                        }

                        var releasedClock = Stopwatch.StartNew();
                        while (
                            releasedClock.ElapsedMilliseconds <
                                readyTimeoutMilliseconds)
                        {
                            ControlledProcessIdentity liveIdentity;
                            ControlledProcessIdentity liveRootIdentity;
                            identityFailure = ValidateControlledChild(
                                process,
                                childProcessId,
                                childIdentity,
                                rootIdentity,
                                out liveIdentity,
                                out liveRootIdentity);
                            if (!String.IsNullOrEmpty(identityFailure))
                            {
                                barrierResult.BarrierFailureReason =
                                    identityFailure == "ReadyChildNotAlive"
                                        ? "ReadyChildNotAliveAfterRelease"
                                        : identityFailure;
                                return barrierResult.BarrierFailureReason;
                            }
                            if (File.Exists(releasedPath))
                            {
                                int releasedProcessId;
                                recordFailure = ReadPidRecord(
                                    releasedPath,
                                    "Released",
                                    out releasedProcessId);
                                if (
                                    !String.IsNullOrEmpty(recordFailure) ||
                                    releasedProcessId != childProcessId)
                                {
                                    barrierResult.BarrierFailureReason =
                                        String.IsNullOrEmpty(recordFailure)
                                            ? "ReleasedPidInvalid"
                                            : recordFailure;
                                    return barrierResult.BarrierFailureReason;
                                }
                                barrierResult.ReleasedObserved = true;
                                barrierResult.ReleasedProcessId =
                                    releasedProcessId;
                                return "";
                            }
                            Thread.Sleep(10);
                        }
                        barrierResult.BarrierFailureReason =
                            "ReleasedTimeout";
                        return barrierResult.BarrierFailureReason;
                    }
                    Thread.Sleep(10);
                }

                barrierResult.ReadyElapsedMilliseconds =
                    readyClock.ElapsedMilliseconds;
                barrierResult.BarrierFailureReason = "ReadyTimeout";
                return barrierResult.BarrierFailureReason;
            };

            Func<Process, Action, string> atTimedWaitActivation =
                (process, activateTimedWait) =>
            {
                try
                {
                    File.WriteAllText(
                        timeoutActivationProbePath,
                        "ACTIVATE");
                }
                catch
                {
                    barrierResult.BarrierFailureReason =
                        "ActivationProbeWriteFailure";
                    return barrierResult.BarrierFailureReason;
                }
                if (barrierMode == "parent-exit-after-released")
                {
                    var parentExitClock = Stopwatch.StartNew();
                    while (
                        parentExitClock.ElapsedMilliseconds <
                            readyTimeoutMilliseconds &&
                        !HasExited(process))
                    {
                        Thread.Sleep(5);
                    }
                }
                if (barrierMode == "exit-after-released")
                {
                    int childProcessId =
                        barrierResult.ReadyProcessId.GetValueOrDefault();
                    try
                    {
                        using (
                            Process exitingChild =
                                Process.GetProcessById(childProcessId))
                        {
                            exitingChild.WaitForExit(
                                readyTimeoutMilliseconds);
                        }
                    }
                    catch
                    {
                        // The process has already exited.
                    }
                }
                var activatedClock = Stopwatch.StartNew();
                while (
                    activatedClock.ElapsedMilliseconds <
                        readyTimeoutMilliseconds)
                {
                    ControlledProcessIdentity finalIdentity;
                    ControlledProcessIdentity finalRootIdentity;
                    string identityFailure = ValidateControlledChild(
                        process,
                        barrierResult.ReadyProcessId.GetValueOrDefault(),
                        childIdentity,
                        rootIdentity,
                        out finalIdentity,
                        out finalRootIdentity);
                    if (!String.IsNullOrEmpty(identityFailure))
                    {
                        barrierResult.BarrierFailureReason =
                            identityFailure == "ReadyChildNotAlive"
                                ? "ReadyChildNotAliveBeforeTimeout"
                                : identityFailure;
                        return barrierResult.BarrierFailureReason;
                    }
                    if (File.Exists(timeoutActivatedPath))
                    {
                        int activatedProcessId;
                        string recordFailure = ReadPidRecord(
                            timeoutActivatedPath,
                            "Activated",
                            out activatedProcessId);
                        if (
                            !String.IsNullOrEmpty(recordFailure) ||
                            activatedProcessId !=
                                barrierResult.ReadyProcessId)
                        {
                            barrierResult.BarrierFailureReason =
                                String.IsNullOrEmpty(recordFailure)
                                    ? "ActivatedPidInvalid"
                                    : recordFailure;
                            return barrierResult.BarrierFailureReason;
                        }
                        ControlledProcessIdentity activatedIdentity;
                        ControlledProcessIdentity activatedRootIdentity;
                        identityFailure = ValidateControlledChild(
                            process,
                            activatedProcessId,
                            childIdentity,
                            rootIdentity,
                            out activatedIdentity,
                            out activatedRootIdentity);
                        if (!String.IsNullOrEmpty(identityFailure))
                        {
                            barrierResult.BarrierFailureReason =
                                identityFailure == "ReadyChildNotAlive"
                                    ? "ReadyChildNotAliveAfterActivated"
                                    : identityFailure;
                            return barrierResult.BarrierFailureReason;
                        }
                        try
                        {
                            File.WriteAllText(
                                timeoutFinalCheckPath,
                                "FINAL_CHECK");
                        }
                        catch
                        {
                            barrierResult.BarrierFailureReason =
                                "FinalCheckProbeWriteFailure";
                            return barrierResult.BarrierFailureReason;
                        }
                        if (
                            barrierMode == "exit-after-activated" ||
                            barrierMode == "exit-after-final-check")
                        {
                            try
                            {
                                using (
                                    Process exitingChild =
                                        Process.GetProcessById(
                                            activatedProcessId))
                                {
                                    exitingChild.WaitForExit(
                                        readyTimeoutMilliseconds);
                                }
                            }
                            catch
                            {
                                // The process has already exited.
                            }
                        }
                        if (barrierMode == "parent-exit-before-timeout")
                        {
                            var parentExitClock = Stopwatch.StartNew();
                            while (
                                parentExitClock.ElapsedMilliseconds <
                                    readyTimeoutMilliseconds &&
                                !HasExited(process))
                            {
                                Thread.Sleep(5);
                            }
                        }
                        identityFailure = ValidateControlledChild(
                            process,
                            activatedProcessId,
                            childIdentity,
                            rootIdentity,
                            out activatedIdentity,
                            out activatedRootIdentity);
                        if (!String.IsNullOrEmpty(identityFailure))
                        {
                            barrierResult.BarrierFailureReason =
                                identityFailure == "ReadyChildNotAlive"
                                    ? "ReadyChildNotAliveBeforeTimeout"
                                    : identityFailure;
                            return barrierResult.BarrierFailureReason;
                        }
                        barrierResult.ActivatedObserved = true;
                        barrierResult.ActivatedProcessId =
                            activatedProcessId;
                        barrierResult.ChildAliveBeforeTimeout = true;
                        barrierResult.TimeoutStartedAfterReady = true;
                        activateTimedWait();
                        return "";
                    }
                    Thread.Sleep(10);
                }
                barrierResult.BarrierFailureReason = "ActivatedTimeout";
                return barrierResult.BarrierFailureReason;
            };

            barrierResult.ProcessResult = RunCore(
                filePath,
                arguments,
                workingDirectory,
                timeoutMilliseconds,
                maximumCharactersPerStream,
                cleanupTimeoutMilliseconds,
                TryTerminate,
                beforeTimedWait,
                atTimedWaitActivation);
            return barrierResult;
        }

        private static BoundedProcessResult RunCore(
            string filePath,
            IReadOnlyList<string> arguments,
            string workingDirectory,
            int timeoutMilliseconds,
            int maximumCharactersPerStream,
            int cleanupTimeoutMilliseconds,
            Func<Process, TerminationOutcome> terminate,
            Func<Process, string> beforeTimedWait,
            Func<Process, Action, string> atTimedWaitActivation)
        {
            var result = new BoundedProcessResult();
            var stdout = new StringBuilder();
            var stderr = new StringBuilder();
            var state = new CaptureState();
            using (var process = new Process())
            {
                var startInfo = new ProcessStartInfo
                {
                    FileName = filePath,
                    WorkingDirectory = workingDirectory,
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    CreateNoWindow = true
                };
                foreach (string argument in arguments)
                {
                    startInfo.ArgumentList.Add(argument);
                }
                process.StartInfo = startInfo;
                result.Attempted = true;

                Task stdoutTask = null;
                Task stderrTask = null;
                try
                {
                    if (!process.Start())
                    {
                        result.FailureReason = "StartFailure";
                        return result;
                    }

                    stdoutTask = CaptureAsync(
                        process.StandardOutput,
                        stdout,
                        maximumCharactersPerStream,
                        "Stdout",
                        process,
                        state,
                        terminate);
                    stderrTask = CaptureAsync(
                        process.StandardError,
                        stderr,
                        maximumCharactersPerStream,
                        "Stderr",
                        process,
                        state,
                        terminate);

                    if (beforeTimedWait != null)
                    {
                        string barrierFailure = beforeTimedWait(process);
                        if (!String.IsNullOrEmpty(barrierFailure))
                        {
                            result.FailureReason = barrierFailure;
                            RecordTermination(state, terminate(process));
                        }
                    }

                    Stopwatch timedWaitClock = null;
                    Action activateTimedWait = () =>
                    {
                        if (timedWaitClock != null)
                        {
                            throw new InvalidOperationException(
                                "Timed wait was activated more than once.");
                        }
                        timedWaitClock = Stopwatch.StartNew();
                    };

                    if (
                        String.IsNullOrEmpty(result.FailureReason) &&
                        atTimedWaitActivation != null)
                    {
                        string barrierFailure =
                            atTimedWaitActivation(
                                process,
                                activateTimedWait);
                        if (!String.IsNullOrEmpty(barrierFailure))
                        {
                            result.FailureReason = barrierFailure;
                            RecordTermination(state, terminate(process));
                        }
                    }

                    if (String.IsNullOrEmpty(result.FailureReason))
                    {
                        if (timedWaitClock == null)
                        {
                            activateTimedWait();
                        }
                        int remainingTimeout = Math.Max(
                            0,
                            timeoutMilliseconds -
                                (int)timedWaitClock.ElapsedMilliseconds);
                        if (
                            remainingTimeout == 0 ||
                            !process.WaitForExit(remainingTimeout))
                        {
                            result.TimedOut = true;
                            result.FailureReason = "Timeout";
                            TerminationOutcome outcome = terminate(process);
                            RecordTermination(state, outcome);
                            if (outcome == TerminationOutcome.BothFailed)
                            {
                                result.FailureReason =
                                    "TerminationFailed";
                            }
                        }
                    }

                    var cleanupClock = Stopwatch.StartNew();
                    if (!HasExited(process))
                    {
                        int remaining = Math.Max(
                            0,
                            cleanupTimeoutMilliseconds -
                                (int)cleanupClock.ElapsedMilliseconds);
                        bool exited = remaining > 0 &&
                            process.WaitForExit(remaining);
                        if (!exited)
                        {
                            TerminationOutcome outcome = terminate(process);
                            RecordTermination(state, outcome);
                            result.FailureReason =
                                outcome == TerminationOutcome.BothFailed
                                    ? "TerminationFailed"
                                    : "CleanupTimeout";
                        }
                    }

                    if (stdoutTask != null && stderrTask != null)
                    {
                        int remaining = Math.Max(
                            0,
                            cleanupTimeoutMilliseconds -
                                (int)cleanupClock.ElapsedMilliseconds);
                        if (
                            remaining == 0 ||
                            !Task.WaitAll(
                                new[] { stdoutTask, stderrTask },
                                remaining))
                        {
                            if (
                                String.IsNullOrEmpty(result.FailureReason) ||
                                result.FailureReason == "Timeout")
                            {
                                result.FailureReason = "CleanupTimeout";
                            }
                            RecordTermination(state, terminate(process));
                        }
                    }

                    if (HasExited(process))
                    {
                        result.ExitCode = process.ExitCode;
                    }

                    lock (state.Sync)
                    {
                        result.OutputLimitExceeded =
                            state.OutputLimitExceeded;
                        if (
                            String.IsNullOrEmpty(result.FailureReason) &&
                            !String.IsNullOrEmpty(state.FailureReason))
                        {
                            result.FailureReason = state.FailureReason;
                        }
                    }

                    if (
                        !result.TimedOut &&
                        !result.OutputLimitExceeded &&
                        String.IsNullOrEmpty(result.FailureReason) &&
                        result.ExitCode.HasValue &&
                        result.ExitCode.Value == 0)
                    {
                        result.Status = "PASS";
                    }
                    else if (
                        String.IsNullOrEmpty(result.FailureReason) &&
                        result.ExitCode.HasValue)
                    {
                        result.FailureReason = "NonZeroExit";
                    }
                }
                catch (Exception exception)
                {
                    RecordTermination(state, terminate(process));
                    if (String.IsNullOrEmpty(result.FailureReason))
                    {
                        result.FailureReason =
                            "StartOrProcessFailure:" +
                            exception.GetType().Name;
                    }
                }
                finally
                {
                    RecordTermination(state, terminate(process));
                    lock (stdout)
                    {
                        result.Stdout = stdout.ToString();
                    }
                    lock (stderr)
                    {
                        result.Stderr = stderr.ToString();
                    }
                }
            }
            return result;
        }
    }
}
'@
}

function ConvertTo-FlashGateArchitecture {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [object] $Architecture
    )

    switch -Regex ([string]$Architecture) {
        '^(?i:x64|amd64|x86_64)$' {
            return 'x64'
        }
        '^(?i:arm64|aarch64)$' {
            return 'arm64'
        }
        default {
            return $null
        }
    }
}

function Get-FlashGateNativeExecutionDecision {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $HostArchitecture,

        [Parameter(Mandatory)]
        [ValidateSet('x64', 'arm64')]
        [string] $TargetArchitecture,

        [switch] $CallerRequestedSkip
    )

    $NormalizedHost = ConvertTo-FlashGateArchitecture `
        -Architecture $HostArchitecture
    if ($null -eq $NormalizedHost) {
        return [pscustomobject]@{
            HostArchitecture        = 'unsupported'
            TargetArchitecture      = $TargetArchitecture
            NativeExecutionEligible = $false
            SkipReason              = 'UnsupportedHostArchitecture'
        }
    }
    if ($NormalizedHost -cne $TargetArchitecture) {
        return [pscustomobject]@{
            HostArchitecture        = $NormalizedHost
            TargetArchitecture      = $TargetArchitecture
            NativeExecutionEligible = $false
            SkipReason              = 'NonNativeTarget'
        }
    }
    if ($CallerRequestedSkip) {
        return [pscustomobject]@{
            HostArchitecture        = $NormalizedHost
            TargetArchitecture      = $TargetArchitecture
            NativeExecutionEligible = $false
            SkipReason              = 'CallerRequestedSkip'
        }
    }
    return [pscustomobject]@{
        HostArchitecture        = $NormalizedHost
        TargetArchitecture      = $TargetArchitecture
        NativeExecutionEligible = $true
        SkipReason              = $null
    }
}

function ConvertTo-FlashGateExecutionState {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [bool] $Attempted,

        [Parameter(Mandatory)]
        [string] $ProcessStatus
    )

    if (-not $Attempted) {
        return 'SKIPPED'
    }
    if ($ProcessStatus -ceq 'PASS') {
        return 'PASS'
    }
    return 'FAIL'
}

function Get-FlashGateMissingHelpLines {
    [CmdletBinding()]
    param(
        [AllowEmptyString()]
        [string] $Output
    )

    $Missing = [System.Collections.Generic.List[string]]::new()
    foreach ($ExpectedLine in @(
        'flashgate-mcp'
        'Usage:'
        'flashgate-mcp --version'
        'flashgate-mcp --version --verbose'
        'flashgate-mcp --help'
        'MCP_ROOT'
        'MCP_READ_ONLY'
        'MCP_ALLOW_CWD_ROOT'
    )) {
        if (-not $Output.Contains(
            $ExpectedLine,
            [StringComparison]::Ordinal
        )) {
            $Missing.Add($ExpectedLine)
        }
    }
    return [string[]]$Missing
}

function Invoke-FlashGateBoundedProcess {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [Parameter(Mandatory)]
        [string[]] $Arguments,

        [Parameter(Mandatory)]
        [string] $WorkingDirectory,

        [ValidateRange(100, 300000)]
        [int] $TimeoutMilliseconds = 10000,

        [ValidateRange(1024, 1048576)]
        [int] $MaximumCharactersPerStream = 65536,

        [ValidateRange(100, 10000)]
        [int] $CleanupTimeoutMilliseconds = 2000
    )

    return [FlashGate.Validation.BoundedProcessRunner]::Run(
        $FilePath,
        [string[]]$Arguments,
        $WorkingDirectory,
        $TimeoutMilliseconds,
        $MaximumCharactersPerStream,
        $CleanupTimeoutMilliseconds
    )
}
