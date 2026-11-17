import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:uuid/uuid.dart';

import 'package:social_finance/core/theme/app_colors.dart';

import '../../data/security_models.dart';
import '../providers/security_providers.dart';

class SecurityQrScannerScreen extends ConsumerStatefulWidget {
  final String actionMode;

  const SecurityQrScannerScreen({super.key, this.actionMode = 'auto'});

  @override
  ConsumerState<SecurityQrScannerScreen> createState() =>
      _SecurityQrScannerScreenState();
}

class _SecurityQrScannerScreenState
    extends ConsumerState<SecurityQrScannerScreen> {
  final MobileScannerController _controller = MobileScannerController(
    detectionSpeed: DetectionSpeed.noDuplicates,
  );
  bool _submitting = false;
  String? _errorBanner;
  DateTime? _lastErrorAt;
  String? _pendingIdempotencyKey;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _submitToken(String rawToken) async {
    final token = rawToken.trim();
    if (token.isEmpty || _submitting) return;
    // Prevent tight retry loops on a stationary expired QR frame
    if (_lastErrorAt != null &&
        DateTime.now().difference(_lastErrorAt!) < const Duration(seconds: 2)) {
      return;
    }

    setState(() {
      _submitting = true;
      _errorBanner = null;
      _pendingIdempotencyKey ??= const Uuid().v4();
    });

    try {
      final api = ref.read(securityApiClientProvider);
      final result = await api.scanQrAttendance(
        qrToken: token,
        action: widget.actionMode,
        idempotencyKey: _pendingIdempotencyKey,
      );
      if (!mounted) return;
      Navigator.of(context).pop<AttendanceScanResult>(result);
    } catch (e) {
      if (!mounted) return;
      _pendingIdempotencyKey = null;
      _lastErrorAt = DateTime.now();
      setState(() {
        _submitting = false;
        _errorBanner = e.toString();
      });
    }
  }

  void _onDetect(BarcodeCapture capture) {
    if (_submitting) return;
    for (final barcode in capture.barcodes) {
      final val = barcode.rawValue;
      if (val != null && val.trim().isNotEmpty) {
        _submitToken(val);
        break;
      }
    }
  }

  Future<void> _openManualTokenDialog() async {
    final ctrl = TextEditingController();
    final submitted = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Input Kode QR Pos'),
        content: TextField(
          controller: ctrl,
          decoration: const InputDecoration(
            labelText: 'Token QR dari Perangkat Pos',
            hintText: 'Tempel kode QR dinamis...',
            border: OutlineInputBorder(),
          ),
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(ctx).pop(ctrl.text.trim()),
            child: const Text('Kirim Absensi'),
          ),
        ],
      ),
    );
    if (submitted != null && submitted.isNotEmpty) {
      await _submitToken(submitted);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Scan QR Pos Keamanan'),
        actions: [
          IconButton(
            tooltip: 'Senter',
            icon: const Icon(Icons.flash_on_outlined),
            onPressed: () => _controller.toggleTorch(),
          ),
          IconButton(
            tooltip: 'Input Manual',
            icon: const Icon(Icons.keyboard_outlined),
            onPressed: _submitting ? null : _openManualTokenDialog,
          ),
        ],
      ),
      body: Stack(
        children: [
          MobileScanner(
            controller: _controller,
            onDetect: _onDetect,
            errorBuilder: (context, error, child) {
              final isDenied =
                  error.errorCode == MobileScannerErrorCode.permissionDenied;
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(
                        Icons.no_photography_outlined,
                        size: 56,
                        color: AppColors.warning,
                      ),
                      const SizedBox(height: 16),
                      Text(
                        isDenied
                            ? 'Izin kamera ditolak. Aktifkan izin kamera pada pengaturan perangkat untuk memindai QR Pos.'
                            : 'Kamera gagal dibuka: ${error.errorDetails?.message ?? error.errorCode.name}',
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 16),
                      OutlinedButton.icon(
                        onPressed: _openManualTokenDialog,
                        icon: const Icon(Icons.keyboard),
                        label: const Text('Masukkan Token Manual'),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
          // Scanner reticle guide
          Center(
            child: Container(
              width: 240,
              height: 240,
              decoration: BoxDecoration(
                border: Border.all(
                  color: _submitting ? AppColors.success : AppColors.accent,
                  width: 3,
                ),
                borderRadius: BorderRadius.circular(16),
              ),
            ),
          ),
          // Top instructions & error alert
          Positioned(
            left: 16,
            right: 16,
            top: 16,
            child: Column(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 14,
                    vertical: 10,
                  ),
                  decoration: BoxDecoration(
                    color: Colors.black.withValues(alpha: 0.7),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: const Text(
                    'Arahkan kamera ke layar QR di Pos Keamanan. QR hanya berlaku sebentar dan diverifikasi di server.',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: Colors.white, fontSize: 13),
                  ),
                ),
                if (_errorBanner != null) ...[
                  const SizedBox(height: 10),
                  Material(
                    color: AppColors.danger,
                    borderRadius: BorderRadius.circular(10),
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Row(
                        children: [
                          const Icon(Icons.error_outline, color: Colors.white),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              _errorBanner!,
                              style: const TextStyle(color: Colors.white),
                            ),
                          ),
                          IconButton(
                            onPressed: () =>
                                setState(() => _errorBanner = null),
                            icon: const Icon(Icons.close, color: Colors.white),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
          if (_submitting)
            Container(
              color: Colors.black.withValues(alpha: 0.55),
              child: const Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    CircularProgressIndicator(color: Colors.white),
                    SizedBox(height: 14),
                    Text(
                      'Memverifikasi absensi ke server...',
                      style: TextStyle(color: Colors.white, fontSize: 15),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
