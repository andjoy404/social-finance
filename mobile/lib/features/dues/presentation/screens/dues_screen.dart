import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../../../core/widgets/empty_state.dart';

class DuesScreen extends StatefulWidget {
  const DuesScreen({super.key});

  @override
  State<DuesScreen> createState() => _DuesScreenState();
}

class _DuesScreenState extends State<DuesScreen> {
  DateTime? _lastBackPressTime;

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        final now = DateTime.now();
        if (_lastBackPressTime == null ||
            now.difference(_lastBackPressTime!) > const Duration(seconds: 2)) {
          _lastBackPressTime = now;
          ScaffoldMessenger.of(context).removeCurrentSnackBar();
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('Tekan sekali lagi untuk keluar dari aplikasi'),
              duration: const Duration(seconds: 2),
            ),
          );
        } else {
          SystemNavigator.pop();
        }
      },
      child: Scaffold(
        appBar: AppBar(title: const Text('Iuran')),
        body: const Center(
          child: EmptyState(
            icon: Icons.receipt_long_outlined,
            title: 'Iuran',
            description:
                'Pengelolaan iuran warga akan tersedia pada tahap berikutnya.',
          ),
        ),
      ),
    );
  }
}
